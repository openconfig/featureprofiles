// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package encapfrr contains utility functions, gRIBI AFT baseline programming,
// table-driven test scenarios, and PCAP packet-distribution validators for
// gRIBI IP-in-IP encapsulation Fast Reroute (FRR) tests (TE-16.2 and TE-16.3).
//
// # 4-Link Topology Architecture (TESTBED_DUT_ATE_4LINKS)
//
//	                  +----------------------------------------+
//	                  |                  DUT                   |
//	ATE port1 <-----> | port1  Bidirectional:                  |
//	                  |        - Rx: test ingress (DSCP 10)    |
//	                  |        - Tx/Rx: IS-IS + iBGP (DEFAULT) |
//	                  |        - Tx: decap fallback (DEFAULT)  |
//	                  |                                        |
//	ATE port2 <------ | port2  Tunnel 1 Primary (203.0.113.1)  |
//	                  |        - NHG#11: NH#11 (w=1, MagicMAC1)|
//	                  |                  NH#12 (w=3, MagicMAC2)|
//	                  |        - NHG#12: NH#13 (w=2, MagicMAC3)|
//	                  |        Baseline share: 25% (1:3:12 MAC)|
//	                  |                                        |
//	ATE port3 <------ | port3  Tunnel 2 Primary (203.0.113.2)  |
//	                  |        - NHG#14: NH#15 (w=1, MagicMAC1)|
//	                  |        Baseline share: 75%             |
//	                  |                                        |
//	ATE port4 <------ | port4  Shared Repair Backup (TE_VRF_222)|
//	                  |        - NHG#13: NH#14 (MagicMAC1)     |
//	                  |          Tunnel 1 Bkp (203.0.113.100)  |
//	                  |        - NHG#15: NH#16 (MagicMAC2)     |
//	                  |          Tunnel 2 Bkp (.101 / .102)    |
//	                  +----------------------------------------+
//
// Multiplexing both backup tunnels onto port4 preserves 100% of FRR failure modes:
//   - Baseline: port2=25%, port3=75%, port4=0%.
//   - Test-1 (port2 DOWN): Tunnel 1 fails over to port4 (25%); Tunnel 2 stays on port3 (75%).
//   - Test-2 (port2 + port4 DOWN): Tunnel 1 decaps out port1 (25%); Tunnel 2 stays on port3 (75%).
//   - Test-3 (NHG#1000=decap, port2 DOWN): 25% decaps out port1; 75% stays on port3.
//   - Test-4 (port2 + port3 DOWN): 100% fails over to port4, where PCAP verifies the 25%:75%
//     split across (203.0.113.100, MagicMAC1) and (203.0.113.101/.102, MagicMAC2).
//   - Test-5..Test-9: 100% exits unencapsulated on port1 via DEFAULT VRF iBGP/IS-IS.
package encapfrr

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4  = 30
	plenIPv48 = 29
	magicIP   = "192.168.1.1"
	magicMAC  = "02:00:00:00:00:01"
	// MagicMAC1 is the destination MAC used for NH#11 (port2), NH#14 (port4), and NH#15 (port3).
	MagicMAC1 = magicMAC
	// MagicMAC2 is the destination MAC used for NH#12 (port2) and NH#16 (port4).
	MagicMAC2 = "02:00:00:00:00:02"
	// MagicMAC3 is the destination MAC used for NH#13 (port2).
	MagicMAC3              = "02:00:00:00:00:03"
	maskLen32              = "32"
	gribiIPv4EntryDefVRF1  = "192.0.2.101"
	gribiIPv4EntryDefVRF2  = "192.0.2.102"
	gribiIPv4EntryDefVRF3  = "192.0.2.103"
	gribiIPv4EntryDefVRF4  = "192.0.2.104"
	gribiIPv4EntryDefVRF5  = "192.0.2.105"
	gribiIPv4EntryVRF1111  = "203.0.113.1"
	gribiIPv4EntryVRF1112  = "203.0.113.2"
	gribiIPv4EntryVRF2221  = "203.0.113.100"
	gribiIPv4EntryVRF2222  = "203.0.113.101"
	gribiIPv4EntryVRF2225  = "203.0.113.102"
	gribiIPv4EntryEncapVRF = "138.0.11.0"
	noMatchEncapDest       = "20.0.0.1"

	// Dedicated /32 next-hop IPs in RFC 2544 (198.18.0.0/15) for 4-link platforms
	// using deviations.GRIBIMACOverrideStaticARPStaticRoute.
	magicNH11IP = "198.18.192.1" // port2, NH#11 (MagicMAC1)
	magicNH12IP = "198.18.192.2" // port2, NH#12 (MagicMAC2)
	magicNH13IP = "198.18.192.3" // port2, NH#13 (MagicMAC3)
	magicNH14IP = "198.18.192.4" // port4, NH#14 (MagicMAC1, Tunnel 1 backup)
	magicNH15IP = "198.18.192.5" // port3, NH#15 (MagicMAC1, Tunnel 2 primary)
	magicNH16IP = "198.18.192.6" // port4, NH#16 (MagicMAC2, Tunnel 2 backup)
)

var (
	// 4-link secondary subnets (single secondary IPv4 subnet per physical egress port)
	// for platforms using deviations.GRIBIMACOverrideWithStaticARP.
	dutPort2Sec4Links = attrs.Attributes{
		Desc:       "dutPort2Sec",
		IPv4Sec:    "192.0.2.33", // 192.0.2.32/29 hosts .34 (NH#11), .35 (NH#12), .36 (NH#13)
		IPv4LenSec: plenIPv48,
	}
	nh11DummyIP4Links = "192.0.2.34"
	nh12DummyIP4Links = "192.0.2.35"
	nh13DummyIP4Links = "192.0.2.36"

	dutPort3Sec4Links = attrs.Attributes{
		Desc:       "dutPort3Sec",
		IPv4Sec:    "192.0.2.41", // 192.0.2.40/30 hosts .42 (NH#15)
		IPv4LenSec: plenIPv4,
	}
	nh15DummyIP4Links = "192.0.2.42"

	dutPort4Sec4Links = attrs.Attributes{
		Desc:       "dutPort4Sec",
		IPv4Sec:    "192.0.2.49", // 192.0.2.48/29 hosts .50 (NH#14), .51 (NH#16)
		IPv4LenSec: plenIPv48,
	}
	nh14DummyIP4Links = "192.0.2.50"
	nh16DummyIP4Links = "192.0.2.51"

	// WantTunnel1MACWeights is [NH#11, NH#12, NH#13] = 1:3:12 on port2.
	WantTunnel1MACWeights = []float64{0.0625, 0.1875, 0.75}
	// WantBackupTunnelWeights is [Tunnel1_Bkp, Tunnel2_Bkp] = 1:3 on port4 in Test-4.
	WantBackupTunnelWeights = []float64{0.25, 0.75}

	// Legacy 8-link variables retained for P4RT-5.3 compatibility.
	dutPort2DummyIP = attrs.Attributes{Desc: "dutPort2", IPv4Sec: "192.0.2.33", IPv4LenSec: plenIPv4}
	otgPort2DummyIP = attrs.Attributes{Desc: "otgPort2", IPv4: "192.0.2.34", IPv4Len: plenIPv4}
	dutPort3DummyIP = attrs.Attributes{Desc: "dutPort3", IPv4Sec: "192.0.2.37", IPv4LenSec: plenIPv4}
	otgPort3DummyIP = attrs.Attributes{Desc: "otgPort3", IPv4: "192.0.2.38", IPv4Len: plenIPv4}
	dutPort4DummyIP = attrs.Attributes{Desc: "dutPort4", IPv4Sec: "192.0.2.41", IPv4LenSec: plenIPv4}
	otgPort4DummyIP = attrs.Attributes{Desc: "otgPort4", IPv4: "192.0.2.42", IPv4Len: plenIPv4}
	dutPort5DummyIP = attrs.Attributes{Desc: "dutPort5", IPv4Sec: "192.0.2.45", IPv4LenSec: plenIPv4}
	otgPort5DummyIP = attrs.Attributes{Desc: "otgPort5", IPv4: "192.0.2.46", IPv4Len: plenIPv4}
	dutPort6DummyIP = attrs.Attributes{Desc: "dutPort5", IPv4Sec: "192.0.2.49", IPv4LenSec: plenIPv4}
	otgPort6DummyIP = attrs.Attributes{Desc: "otgPort5", IPv4: "192.0.2.50", IPv4Len: plenIPv4}
	dutPort7DummyIP = attrs.Attributes{Desc: "dutPort5", IPv4Sec: "192.0.2.53", IPv4LenSec: plenIPv4}
	otgPort7DummyIP = attrs.Attributes{Desc: "otgPort5", IPv4: "192.0.2.54", IPv4Len: plenIPv4}
)

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

// configStaticArp configures static arp entries.
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	return configStaticArps(p, map[string]string{ipv4addr: macAddr})
}

// StaticARPWithSpecificIP4Links configures secondary subnets and static ARP entries
// across port2 (3 MACs), port3 (1 MAC), and port4 (2 MACs). Secondary IPv4 subnets are
// committed before static ARP entries to avoid duplicate path updates on Cisco IOS-XR.
func StaticARPWithSpecificIP4Links(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2").Name()
	p3 := dut.Port(t, "port3").Name()
	p4 := dut.Port(t, "port4").Name()

	gnmi.Update(t, dut, gnmi.OC().Interface(p2).Config(), dutPort2Sec4Links.NewOCInterface(p2, dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3).Config(), dutPort3Sec4Links.NewOCInterface(p3, dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4).Config(), dutPort4Sec4Links.NewOCInterface(p4, dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2).Config(), configStaticArps(p2, map[string]string{
		nh11DummyIP4Links: MagicMAC1,
		nh12DummyIP4Links: MagicMAC2,
		nh13DummyIP4Links: MagicMAC3,
	}))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3).Config(), configStaticArps(p3, map[string]string{
		nh15DummyIP4Links: MagicMAC1,
	}))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4).Config(), configStaticArps(p4, map[string]string{
		nh14DummyIP4Links: MagicMAC1,
		nh16DummyIP4Links: MagicMAC2,
	}))
}

// StaticARPWithMagicUniversalIP4Links programs 6 dedicated /32 interface static routes
// and static ARP entries across port2 (3 NHs), port3 (1 NH), and port4 (2 NHs).
func StaticARPWithMagicUniversalIP4Links(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2").Name()
	p3 := dut.Port(t, "port3").Name()
	p4 := dut.Port(t, "port4").Name()

	type staticBinding struct {
		ip   string
		port string
		mac  string
	}
	bindings := []staticBinding{
		{ip: magicNH11IP, port: p2, mac: MagicMAC1},
		{ip: magicNH12IP, port: p2, mac: MagicMAC2},
		{ip: magicNH13IP, port: p2, mac: MagicMAC3},
		{ip: magicNH14IP, port: p4, mac: MagicMAC1},
		{ip: magicNH15IP, port: p3, mac: MagicMAC1},
		{ip: magicNH16IP, port: p4, mac: MagicMAC2},
	}

	sb := &gnmi.SetBatch{}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut),
	)
	arpsByPort := make(map[string]map[string]string)
	for _, b := range bindings {
		pfx := b.ip + "/32"
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(pfx),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				"0": {
					Index: ygot.String("0"),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(b.port),
					},
				},
			},
		}
		gnmi.BatchUpdate(sb, sp.Static(pfx).Config(), s)
		if arpsByPort[b.port] == nil {
			arpsByPort[b.port] = make(map[string]string)
		}
		arpsByPort[b.port][b.ip] = b.mac
	}
	for _, portName := range []string{p2, p3, p4} {
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(portName).Config(), configStaticArps(portName, arpsByPort[portName]))
	}
	sb.Set(t, dut)
}

// ConfigureBaseGribiRoutes4Links installs NH#11..NH#16, NHG#11..NHG#15, and
// 192.0.2.101..105/32 in DEFAULT VRF across port2, port3, and port4.
func ConfigureBaseGribiRoutes4Links(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()
	defNI := deviations.DefaultNetworkInstance(dut)
	p2 := dut.Port(t, "port2").Name()
	p3 := dut.Port(t, "port3").Name()
	p4 := dut.Port(t, "port4").Name()

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(11).WithMacAddress(MagicMAC1).WithInterfaceRef(p2).WithIPAddress(magicNH11IP),
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(12).WithMacAddress(MagicMAC2).WithInterfaceRef(p2).WithIPAddress(magicNH12IP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(13).WithMacAddress(MagicMAC3).WithInterfaceRef(p2).WithIPAddress(magicNH13IP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(14).WithMacAddress(MagicMAC1).WithInterfaceRef(p4).WithIPAddress(magicNH14IP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(15).WithMacAddress(MagicMAC1).WithInterfaceRef(p3).WithIPAddress(magicNH15IP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(16).WithMacAddress(MagicMAC2).WithInterfaceRef(p4).WithIPAddress(magicNH16IP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(15).AddNextHop(16, 1),
		)
	} else if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(11).WithMacAddress(MagicMAC1).WithInterfaceRef(p2).WithIPAddress(nh11DummyIP4Links),
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(12).WithMacAddress(MagicMAC2).WithInterfaceRef(p2).WithIPAddress(nh12DummyIP4Links),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(13).WithMacAddress(MagicMAC3).WithInterfaceRef(p2).WithIPAddress(nh13DummyIP4Links),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(14).WithMacAddress(MagicMAC1).WithInterfaceRef(p4).WithIPAddress(nh14DummyIP4Links),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(15).WithMacAddress(MagicMAC1).WithInterfaceRef(p3).WithIPAddress(nh15DummyIP4Links),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(16).WithMacAddress(MagicMAC2).WithInterfaceRef(p4).WithIPAddress(nh16DummyIP4Links),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(15).AddNextHop(16, 1),
		)
	} else {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(11).WithMacAddress(MagicMAC1).WithInterfaceRef(p2),
			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(12).WithMacAddress(MagicMAC2).WithInterfaceRef(p2),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(13).WithMacAddress(MagicMAC3).WithInterfaceRef(p2),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(14).WithMacAddress(MagicMAC1).WithInterfaceRef(p4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(15).WithMacAddress(MagicMAC1).WithInterfaceRef(p3),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defNI).WithIndex(16).WithMacAddress(MagicMAC2).WithInterfaceRef(p4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defNI).WithID(15).AddNextHop(16, 1),
		)
	}
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),
		fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),
		fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),
		fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),
		fluent.IPv4Entry().WithNetworkInstance(defNI).WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

func normalizeCounts(xs []uint64) ([]float64, uint64) {
	var sum uint64
	for _, x := range xs {
		sum += x
	}
	ys := make([]float64, len(xs))
	if sum == 0 {
		return ys, 0
	}
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// ValidateTunnel1MACDistribution verifies that packets encapsulated to 203.0.113.1
// on port2 follow the 1:3:12 hierarchical wECMP split across MagicMAC1..MagicMAC3.
func ValidateTunnel1MACDistribution(pcapPath string, tolerance float64) error {
	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		return fmt.Errorf("opening port2 pcap %q: %v", pcapPath, err)
	}
	defer handle.Close()

	macCounts := make([]uint64, 3)
	macIndex := map[string]int{
		MagicMAC1: 0,
		MagicMAC2: 1,
		MagicMAC3: 2,
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
	gotWeights, total := normalizeCounts(macCounts)
	if total == 0 {
		return fmt.Errorf("ValidateTunnel1MACDistribution(port2): got 0 encapsulated packets for %s", gribiIPv4EntryVRF1111)
	}
	if diff := cmp.Diff(WantTunnel1MACWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		return fmt.Errorf("ValidateTunnel1MACDistribution(port2) counts=%v ratios=%v, want %v (-want +got):\n%s",
			macCounts, gotWeights, WantTunnel1MACWeights, diff)
	}
	return nil
}

// ValidateBackupTunnelDistribution verifies that shared backup port4 in Test-4
// carries both Tunnel 1 Backup (203.0.113.100, MagicMAC1) and Tunnel 2 Backup
// (203.0.113.101[/.102], MagicMAC2) in the expected 25% : 75% ratio.
func ValidateBackupTunnelDistribution(pcapPath, innerDst string, allowSecondaryT2OuterIP bool, tolerance float64) error {
	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		return fmt.Errorf("opening port4 backup pcap %q: %v", pcapPath, err)
	}
	defer handle.Close()

	var bkpCounts [2]uint64
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ethLayer := packet.Layer(layers.LayerTypeEthernet)
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ethLayer == nil || ipLayer == nil {
			continue
		}
		eth, _ := ethLayer.(*layers.Ethernet)
		outerIP, _ := ipLayer.(*layers.IPv4)
		innerPkt := gopacket.NewPacket(outerIP.Payload, outerIP.NextLayerType(), gopacket.Default)
		innerLayer := innerPkt.Layer(layers.LayerTypeIPv4)
		if innerLayer == nil {
			continue
		}
		innerIP, _ := innerLayer.(*layers.IPv4)
		if innerIP.DstIP.String() != innerDst {
			continue
		}

		dstIP := outerIP.DstIP.String()
		dstMAC := eth.DstMAC.String()
		isT2Outer := dstIP == gribiIPv4EntryVRF2222 || (allowSecondaryT2OuterIP && dstIP == gribiIPv4EntryVRF2225)
		switch {
		case dstIP == gribiIPv4EntryVRF2221 && dstMAC == MagicMAC1:
			bkpCounts[0]++
		case isT2Outer && dstMAC == MagicMAC2:
			bkpCounts[1]++
		default:
			return fmt.Errorf("ValidateBackupTunnelDistribution(port4): unexpected outerDst=%s dstMAC=%s", dstIP, dstMAC)
		}
	}
	gotWeights, total := normalizeCounts(bkpCounts[:])
	if total == 0 {
		return fmt.Errorf("ValidateBackupTunnelDistribution(port4): got 0 IP-in-IP backup packets for inner DstIP %s", innerDst)
	}
	if diff := cmp.Diff(WantBackupTunnelWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		return fmt.Errorf("ValidateBackupTunnelDistribution(port4) counts=%v ratios=%v, want %v (-want +got):\n%s",
			bkpCounts, gotWeights, WantBackupTunnelWeights, diff)
	}
	return nil
}

func programAftWithDummyIP(t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()).
			WithIPAddress(otgPort2DummyIP.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()).
			WithIPAddress(otgPort3DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()).
			WithIPAddress(otgPort4DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(12).AddNextHop(13, 2),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()).
			WithIPAddress(otgPort5DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(13).AddNextHop(14, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()).
			WithIPAddress(otgPort6DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(14).AddNextHop(15, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()).
			WithIPAddress(otgPort7DummyIP.IPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(15).AddNextHop(16, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
}

// StaticARPWithSpecificIP configures secondary IPs and static ARP on 8-link testbeds (retained for P4RT-5.3).
func StaticARPWithSpecificIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), dutPort5DummyIP.NewOCInterface(p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p6.Name()).Config(), dutPort6DummyIP.NewOCInterface(p6.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p7.Name()).Config(), dutPort7DummyIP.NewOCInterface(p7.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p6.Name()).Config(), configStaticArp(p6.Name(), otgPort6DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p7.Name()).Config(), configStaticArp(p7.Name(), otgPort7DummyIP.IPv4, magicMAC))
}

// StaticARPWithMagicUniversalIP programs static ARP with magic universal IP on 8-link testbeds (retained for P4RT-5.3).
func StaticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
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

// ConfigureBaseGribiRoutes programs base gRIBI routes on 8-link testbeds (retained for P4RT-5.3).
func ConfigureBaseGribiRoutes(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	// Programming AFT entries for prefixes in DEFAULT VRF
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()).WithIPAddress(magicIP),
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(15).AddNextHop(16, 1),
		)
	} else if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		programAftWithDummyIP(t, dut, client)
	} else {
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port2").Name()),
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(12).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port3").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(13).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port4").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port5").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port6").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(dut.Port(t, "port7").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(15).AddNextHop(16, 1),
		)
	}
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// TestCase is a struct to hold the parameters for FRR test cases.
// LoadBalancePercent specifies the expected share across [port1, port2, port3, port4].
type TestCase struct {
	Desc                   string
	DownPortList           []string
	CapturePortList        []string
	EncapHeaderOuterIPList []string
	EncapHeaderInnerIPList []string
	TrafficDestIP          string
	LoadBalancePercent     []float64
	TestID                 string
}

// TestCases returns a list of base test cases for 4-link FRR tests ([port1, port2, port3, port4]).
func TestCases(atePortNamelist []string, ipv4InnerDst string) []*TestCase {
	cases := []*TestCase{
		{
			Desc:                   "Test-1: primary encap unviable but backup encap viable for single tunnel",
			DownPortList:           []string{"port2"},
			CapturePortList:        []string{atePortNamelist[3], atePortNamelist[2]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst, ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0.75, 0.25},
			TestID:                 "primarySingle",
		}, {
			Desc:                   "Test-2: primary and backup encap unviable for single tunnel",
			DownPortList:           []string{"port2", "port4"},
			CapturePortList:        []string{atePortNamelist[2], atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0.25, 0, 0.75, 0},
			TestID:                 "primaryBackupSingle",
		}, {
			Desc:                   "Test-3: primary encap unviable with backup to routing for single tunnel",
			DownPortList:           []string{"port2"},
			CapturePortList:        []string{atePortNamelist[2], atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0.25, 0, 0.75, 0},
			TestID:                 "primaryBackupRoutingSingle",
		}, {
			Desc:                   "Test-4: primary encap unviable but backup encap viable for all tunnels",
			DownPortList:           []string{"port2", "port3"},
			CapturePortList:        []string{atePortNamelist[3]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 1},
			TestID:                 "primaryAll",
		}, {
			Desc:                   "Test-5: primary and backup encap unviable for all tunnels",
			DownPortList:           []string{"port2", "port3", "port4"},
			CapturePortList:        []string{atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0},
			TestID:                 "primaryBackupAll",
		}, {
			Desc:                   "Test-6: primary encap unviable with backup to routing for all tunnels",
			DownPortList:           []string{"port2", "port3"},
			CapturePortList:        []string{atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0},
			TestID:                 "primaryBackupRoutingAll",
		}, {
			Desc:                   "Test-7: no match in encap VRF",
			DownPortList:           []string{},
			CapturePortList:        []string{atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{noMatchEncapDest},
			TrafficDestIP:          noMatchEncapDest,
			LoadBalancePercent:     []float64{1, 0, 0, 0},
			TestID:                 "encapNoMatch",
		},
	}

	return cases
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
