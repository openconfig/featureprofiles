// Copyright 2026 Google LLC
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

package bgpdrainpolicy_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	routeCount        = 10000
	routeV4Start      = "198.18.0.0"
	routeV6Start      = "2001:db8:1::1"
	routeV4Prefix     = 32
	routeV6Prefix     = 128
	denyAllPolicy     = "DENY_ALL"
	v4FlowName        = "drain-v4"
	v6FlowName        = "drain-v6"
	trafficFrameSize  = 512
	bgpPort           = 179
	nonExistentPolicy = "NON_EXISTENT_POLICY"
	queryWaitTime     = 60
)

type flowKey struct{ src, dst string }

type routingPolicy struct {
	policyName string
	action     oc.E_RoutingPolicy_PolicyResultType
	statement  string
}

type wantOTGPrefixes map[string]uint64

func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, params routingPolicy) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(params.policyName)
	stmt, err := pd.AppendNewStatement(params.statement)
	if err != nil {
		t.Fatalf("AppendNewStatement() failed: %v", err)
	}
	stmt.GetOrCreateActions().PolicyResult = params.action
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configureDUTBGP(t *testing.T, as uint32, routerID string, nbrs []*cfgplugins.BgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	t.Helper()
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
	bgp := niProto.GetOrCreateBgp()

	// Global Configuration using cfgplugins
	globalOpts := []cfgplugins.GlobalOption{
		cfgplugins.WithAS(as),
		cfgplugins.WithRouterID(routerID),
	}
	cfgplugins.ConfigureGlobal(bgp, dut, globalOpts...)

	for pgName, afiSafiList := range map[string][]oc.E_BgpTypes_AFI_SAFI_TYPE{
		cfgplugins.BGPPeerGroup1: {oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		cfgplugins.BGPPeerGroup2: {oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
	} {
		peerGroup := bgp.GetOrCreatePeerGroup(pgName)
		pgOpts := []cfgplugins.PeerGroupOption{
			cfgplugins.WithPeerAS(cfgplugins.AteAS1),
			cfgplugins.WithPGDescription(pgName),
		}
		for _, afiSafi := range afiSafiList {
			pgOpts = append(pgOpts, cfgplugins.WithPGAfiSafiEnabled(afiSafi, true, false))
		}
		cfgplugins.ConfigurePeerGroup(peerGroup, dut, pgOpts...)
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		cfgplugins.ConfigurePeer(bgpNbr, dut,
			cfgplugins.WithPeerGroup(nbr.PeerGrp, nbr.PeerAS, "", "", false),
			cfgplugins.WithPeerAfiSafiEnabled(nbr.IsV4, "", "", false),
		)
	}
	return niProto
}

func addPeers(dev gosnappi.Device, port *attrs.Attributes, routes bool) {
	eth := dev.Ethernets().Items()[0]
	ip4, ip6 := eth.Ipv4Addresses().Items()[0], eth.Ipv6Addresses().Items()[0]

	v4 := otgconfighelpers.AddBGPV4Peer(dev, ip4.Name(),
		otgconfighelpers.WithBGPName(port.Name+".BGP4.peer"),
		otgconfighelpers.WithBGPPeerAddress(ip4.Gateway()),
		otgconfighelpers.WithBGPASNumber(cfgplugins.AteAS1),
		otgconfighelpers.WithBGPEBGP(),
		otgconfighelpers.WithBGPRouterID(port.IPv4),
		otgconfighelpers.WithoutBGPGR(),
		otgconfighelpers.WithBGPLearnedV4Pfx(true))
	v6 := otgconfighelpers.AddBGPV6Peer(dev, ip6.Name(),
		otgconfighelpers.WithBGPName(port.Name+".BGP6.peer"),
		otgconfighelpers.WithBGPPeerAddress(ip6.Gateway()),
		otgconfighelpers.WithBGPASNumber(cfgplugins.AteAS1),
		otgconfighelpers.WithBGPEBGP(),
		otgconfighelpers.WithoutBGPGR(),
		otgconfighelpers.WithBGPLearnedV6Pfx(true))

	if routes {
		otgconfighelpers.AddBGPV4Routes(v4, port.Name+".BGP4.peer.route",
			[]string{fmt.Sprintf("%s/%d", routeV4Start, routeV4Prefix)},
			otgconfighelpers.WithBGPRouteAddressCount(routeCount))
		otgconfighelpers.AddBGPV6Routes(v6, port.Name+".BGP6.peer.route",
			[]string{fmt.Sprintf("%s/%d", routeV6Start, routeV6Prefix)},
			otgconfighelpers.WithBGPRouteAddressCount(routeCount))
	}
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()
	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)

	dev1 := devices[0]
	dev2 := devices[1]

	addPeers(dev1, bs.ATEPorts[0], true)
	addPeers(dev2, bs.ATEPorts[1], false)

	ip2v4 := dev2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2v6 := dev2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	eth2 := dev2.Ethernets().Items()[0]

	flow4 := bs.ATETop.Flows().Add().SetName(v4FlowName)
	flow4.Metrics().SetEnable(true)
	flow4.TxRx().Device().SetTxNames([]string{ip2v4.Name()}).SetRxNames([]string{bs.ATEPorts[0].Name + ".BGP4.peer.route"})
	flow4.Duration().Continuous()
	flow4.Size().SetFixed(trafficFrameSize)
	e4 := flow4.Packet().Add().Ethernet()
	e4.Src().SetValue(eth2.Mac())
	v4 := flow4.Packet().Add().Ipv4()
	v4.Src().SetValue(ip2v4.Address())
	v4.Dst().Increment().SetStart(routeV4Start).SetCount(routeCount)

	flow6 := bs.ATETop.Flows().Add().SetName(v6FlowName)
	flow6.Metrics().SetEnable(true)
	flow6.TxRx().Device().SetTxNames([]string{ip2v6.Name()}).SetRxNames([]string{bs.ATEPorts[0].Name + ".BGP6.peer.route"})
	flow6.Duration().Continuous()
	flow6.Size().SetFixed(trafficFrameSize)
	e6 := flow6.Packet().Add().Ethernet()
	e6.Src().SetValue(eth2.Mac())
	v6 := flow6.Packet().Add().Ipv6()
	v6.Src().SetValue(ip2v6.Address())
	v6.Dst().Increment().SetStart(routeV6Start).SetCount(routeCount)
}

func verifyPrefixes(t *testing.T, dut *ondatra.DUTDevice, nbr string, afi oc.E_BgpTypes_AFI_SAFI_TYPE, prefixCount uint32, verifyPrefixesSent bool, verifyPrefixesReceived bool) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
	if verifyPrefixesSent {
		sentQuery := bgpPath.Neighbor(nbr).AfiSafi(afi).Prefixes().Sent().State()
		if _, ok := gnmi.Watch(t, dut, sentQuery, queryWaitTime*time.Second, func(val *ygnmi.Value[uint32]) bool {
			v, present := val.Val()
			if present && v == prefixCount {
				t.Logf("peer %s: prefixes sent as expected, got: %d", nbr, v)
				return true
			}
			return false
		}).Await(t); !ok {
			t.Errorf("prefixes sent error: timeout waiting for %d prefixes on neighbor %s", prefixCount, nbr)
		}
	}
	if verifyPrefixesReceived {
		rcvdQuery := bgpPath.Neighbor(nbr).AfiSafi(afi).Prefixes().Received().State()
		if _, ok := gnmi.Watch(t, dut, rcvdQuery, queryWaitTime*time.Second, func(val *ygnmi.Value[uint32]) bool {
			v, present := val.Val()
			if present && v == prefixCount {
				t.Logf("peer %s: prefixes received as expected, got: %d", nbr, v)
				return true
			}
			return false
		}).Await(t); !ok {
			t.Errorf("prefixes received error: timeout waiting for %d prefixes on neighbor %s", prefixCount, nbr)
		}
	}
}

func applyExportPolicy(t *testing.T, dut *ondatra.DUTDevice, nbr string, afi oc.E_BgpTypes_AFI_SAFI_TYPE, policy string) {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
	gnmi.Replace(t, dut, bgpPath.Neighbor(nbr).AfiSafi(afi).ApplyPolicy().ExportPolicy().Config(), []string{policy})
}

func verifyTrafficState(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	for _, flow := range top.Flows().Items() {
		otgutils.ExpectedTrafficLoss(t, ate.OTG(), flow.Name(), 0, 1)
	}
}

func removeExportPolicy(t *testing.T, dut *ondatra.DUTDevice, nbr string, afi oc.E_BgpTypes_AFI_SAFI_TYPE) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
	gnmi.Delete(t, dut, bgpPath.Neighbor(nbr).AfiSafi(afi).ApplyPolicy().ExportPolicy().Config())
}

func parsePathAttrs(b []byte) (withdrawn int) {
	for len(b) >= 2 {
		flags := b[0]
		attrType := b[1]
		b = b[2:]

		var attrLen int
		if flags&0x10 != 0 { // Extended Length
			if len(b) < 2 {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(b[0:2]))
			b = b[2:]
		} else {
			if len(b) < 1 {
				break
			}
			attrLen = int(b[0])
			b = b[1:]
		}

		if attrLen > len(b) {
			break
		}
		attrVal := b[:attrLen]
		b = b[attrLen:]

		// MP_UNREACH_NLRI (Type 15): AFI (2 bytes), SAFI (1 byte), Withdrawn Routes (NLRI)
		if attrType == 15 && len(attrVal) >= 3 {
			withdrawn += parsePrefixes(attrVal[3:])
		}
	}
	return withdrawn
}

func countBGPWithdrawnRoutes(t *testing.T, ate *ondatra.ATEDevice, portName string) int {
	t.Helper()
	packetBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portName))
	// Write capture to temporary pcap file for analysis
	f, err := os.CreateTemp("", ".pcap")
	if err != nil {
		t.Fatalf("Could not create temporary pcap file: %v", err)
	}
	if _, err := f.Write(packetBytes); err != nil {
		t.Fatalf("Could not write packetBytes to pcap file: %v", err)
	}
	defer os.Remove(f.Name()) // Clean up the temporary file
	f.Close()

	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		t.Fatalf("Could not open pcap file: %v", err)
	}
	defer handle.Close()

	streams := map[flowKey][]byte{}
	var totalWithdrawn int

	for packet := range gopacket.NewPacketSource(handle, handle.LinkType()).Packets() {
		var srcIP, dstIP string
		if ip, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4); ok {
			srcIP, dstIP = ip.SrcIP.String(), ip.DstIP.String()
		} else if ip6, ok := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6); ok {
			srcIP, dstIP = ip6.SrcIP.String(), ip6.DstIP.String()
		} else {
			continue
		}

		tcp, ok := packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
		if !ok || (tcp.SrcPort != bgpPort && tcp.DstPort != bgpPort) {
			continue
		}
		key := flowKey{srcIP, dstIP}
		streams[key] = append(streams[key], tcp.LayerPayload()...)

		buf := streams[key]
		for len(buf) >= 19 {
			msgLen := int(binary.BigEndian.Uint16(buf[16:18]))
			if msgLen < 19 || msgLen > len(buf) {
				break // wait for more segments to arrive
			}
			if buf[18] == 2 && msgLen >= 21 { // UPDATE message
				wLen := int(binary.BigEndian.Uint16(buf[19:21]))
				if 21+wLen <= msgLen {
					// Traditional IPv4 withdrawn routes
					totalWithdrawn += parsePrefixes(buf[21 : 21+wLen])

					// Total Path Attributes Length
					rest := buf[21+wLen : msgLen]
					if len(rest) >= 2 {
						attrLen := int(binary.BigEndian.Uint16(rest[0:2]))
						if 2+attrLen <= len(rest) {
							// MP-BGP IPv6 withdrawn routes in MP_UNREACH_NLRI (Type 15)
							totalWithdrawn += parsePathAttrs(rest[2 : 2+attrLen])
						}
					}
				}
			}
			buf = buf[msgLen:]
		}
		streams[key] = buf
	}
	t.Logf("total withdrawn prefixes on %s = %d", portName, totalWithdrawn)
	return totalWithdrawn
}

func parsePrefixes(b []byte) (count int) {
	for len(b) > 0 {
		byteLen := (int(b[0]) + 7) / 8
		if 1+byteLen > len(b) {
			break
		}
		count++
		b = b[1+byteLen:]
	}
	return count
}

func verifyOTGPrefixes(t *testing.T, bs *cfgplugins.BGPSession, portName string, expectedRoutes wantOTGPrefixes, inRoutes bool, withdrawRoutes bool) {
	t.Helper()
	var queryPath ygnmi.SingletonQuery[uint64]
	for peerName, want := range expectedRoutes {
		var matched bool
		var lastInWithdraws uint64
		var msgPath string
		if inRoutes {
			queryPath = gnmi.OTG().BgpPeer(peerName).Counters().InRoutes().State()
			msgPath = "routes received"
		}
		if withdrawRoutes {
			queryPath = gnmi.OTG().BgpPeer(peerName).Counters().InRouteWithdraw().State()
			msgPath = "route withdraw"
		}
		_, ok := gnmi.Watch(t, bs.ATE.OTG(), queryPath, queryWaitTime*time.Second, func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			if present {
				lastInWithdraws = v
			}
			if want == 0 {
				matched = v == want
			} else {
				matched = v >= want
			}
			if matched {
				t.Logf("peer %s: %s on port %s: %d, want %d", peerName, msgPath, portName, v, want)
				return true
			}
			return false
		}).Await(t)

		if !ok {
			t.Errorf("failed: peer %s: %s on port %s: %d, want %d", peerName, msgPath, portName, lastInWithdraws, want)
		}
	}
}

func TestBGPDrainPolicy(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount2, nil)
	dut := bs.DUT
	ate := bs.ATE
	ateP1 := bs.ATEPorts[0]
	ateP2 := bs.ATEPorts[1]
	batch := &gnmi.SetBatch{}

	t.Cleanup(func() {
		t.Log("Cleaning up BGP and routing policy config on DUT and stopping OTG protocols")
		ate.OTG().StopProtocols(t)
		removeExportPolicy(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		removeExportPolicy(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(denyAllPolicy).Config())
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config())
	})

	t.Log("configure DUT interfaces")
	if err := bs.PushDUT(t); err != nil {
		t.Fatalf("PushDUT() failed: %v", err)
	}

	t.Log("configure BGP on DUT")
	nbrs := []*cfgplugins.BgpNeighbor{
		{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateP1.IPv4, IsV4: true, PeerGrp: cfgplugins.BGPPeerGroup1},
		{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateP1.IPv6, IsV4: false, PeerGrp: cfgplugins.BGPPeerGroup1},
		{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateP2.IPv4, IsV4: true, PeerGrp: cfgplugins.BGPPeerGroup2},
		{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateP2.IPv6, IsV4: false, PeerGrp: cfgplugins.BGPPeerGroup2},
	}
	niProto := configureDUTBGP(t, cfgplugins.DutAS, bs.DUTPorts[0].IPv4, nbrs, dut)
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config(), niProto)
	batch.Set(t, dut)

	t.Log("configuring deny-all routing policy on DUT")
	configureRoutingPolicy(t, dut, routingPolicy{
		policyName: denyAllPolicy,
		statement:  "deny-all-statement",
		action:     oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
	})

	t.Log("configure bgp on ATE")
	configureOTG(t, bs)

	bgpV4Peer2 := bs.ATETop.Devices().Items()[1].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0].Name()
	bgpV6Peer2 := bs.ATETop.Devices().Items()[1].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0].Name()

	bs.ATETop.Captures().Add().SetName("packetCapture").SetPortNames([]string{bs.ATEPorts[1].Name}).SetFormat(gosnappi.CaptureFormat.PCAP)
	bs.PushAndStartATE(t)

	otgutils.WaitForARP(t, ate.OTG(), bs.ATETop, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), bs.ATETop, "IPv6")

	t.Log("validate BGP session is established on DUT and ATE")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Log("start traffic from port2 to ate bgp peer routes")
	ate.OTG().StartTraffic(t)

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Verify baseline Prefix Advertisement and Traffic",
			run: func(t *testing.T) {
				verifyTrafficState(t, ate, bs.ATETop)
				t.Log("verify DUT received prefixes from port1 and sent to port2")
				verifyPrefixes(t, dut, ateP1.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, routeCount, false, true)
				verifyPrefixes(t, dut, ateP1.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, false, true)
				verifyPrefixes(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, routeCount, true, false)
				verifyPrefixes(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, true, false)

				verifyOTGPrefixes(t, bs, bs.ATEPorts[1].Name, wantOTGPrefixes{bgpV4Peer2: uint64(routeCount), bgpV6Peer2: uint64(routeCount)}, true, false)
			},
		},
		{
			name: "Apply Deny All Export Policy",
			run: func(t *testing.T) {
				bs.ATE.OTG().StopTraffic(t)

				cs := gosnappi.NewControlState()
				cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
				bs.ATE.OTG().SetControlState(t, cs)

				bs.ATE.OTG().StartProtocols(t)
				cfgplugins.VerifyDUTBGPEstablished(t, dut)
				cfgplugins.VerifyOTGBGPEstablished(t, ate)

				t.Log("apply DENY_ALL export policy toward ATE port2 for IPv4 and IPv6")
				applyExportPolicy(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, denyAllPolicy)
				applyExportPolicy(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, denyAllPolicy)

				t.Log("verify DUT sent prefixes to port2 drop to 0")
				verifyPrefixes(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, 0, true, false)
				verifyPrefixes(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, 0, true, false)

				// Sleep to get the packet capture populated
				time.Sleep(10 * time.Second)
				cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
				bs.ATE.OTG().SetControlState(t, cs)

				withdrawRoutesDUT := countBGPWithdrawnRoutes(t, bs.ATE, bs.ATEPorts[1].Name)
				if withdrawRoutesDUT < 2*routeCount {
					t.Errorf("failed: expected %d BGP withdrawn routes on port2 (IPv4 + IPv6), got %d", 2*routeCount, withdrawRoutesDUT)
				}

				verifyOTGPrefixes(t, bs, bs.ATEPorts[1].Name, wantOTGPrefixes{
					bgpV4Peer2: uint64(routeCount),
					bgpV6Peer2: uint64(routeCount),
				}, false, true)
			},
		},
		{
			name: "Remove Policy and Restore",
			run: func(t *testing.T) {
				t.Log("remove export policy from ATE port2 neighbor for both AFI/SAFI")
				removeExportPolicy(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				removeExportPolicy(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)

				ate.OTG().StartTraffic(t)

				t.Log("wait for convergence and verify re-advertisement to 10,000")
				verifyPrefixes(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, routeCount, true, false)
				verifyPrefixes(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, true, false)

				t.Log("verify traffic recovery with near 0% loss")
				verifyTrafficState(t, ate, bs.ATETop)

				ate.OTG().StopTraffic(t)
				// To reset the counters on OTG
				ate.OTG().StopProtocols(t)
				verifyOTGPrefixes(t, bs, bs.ATEPorts[1].Name, wantOTGPrefixes{
					bgpV4Peer2: uint64(0),
					bgpV6Peer2: uint64(0),
				}, true, false)
			},
		},
		{
			name: "Apply AFI/SAFI Specific Policy",
			run: func(t *testing.T) {
				bs.ATE.OTG().StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), bs.ATETop, "IPv4")
				otgutils.WaitForARP(t, ate.OTG(), bs.ATETop, "IPv6")

				verifyOTGPrefixes(t, bs, bs.ATEPorts[1].Name, wantOTGPrefixes{
					bgpV4Peer2: uint64(routeCount),
					bgpV6Peer2: uint64(routeCount),
				}, true, false)

				t.Log("apply DENY_ALL export policy only to IPv4 AFI/SAFI on ATE port2 neighbor")
				applyExportPolicy(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, denyAllPolicy)

				t.Log("verify IPv4 sent prefixes go to 0 while IPv6 remains at 10,000")
				verifyPrefixes(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, 0, true, false)
				verifyPrefixes(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, true, false)

				verifyOTGPrefixes(t, bs, bs.ATEPorts[1].Name, wantOTGPrefixes{
					bgpV4Peer2: uint64(routeCount),
					bgpV6Peer2: uint64(0),
				}, false, true)

				removeExportPolicy(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			},
		},
		{
			name: "RT-7.12.5 Apply Non Existent Policy",
			run: func(t *testing.T) {
				policies := gnmi.LookupAll(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinitionAny().Name().State())
				for _, p := range policies {
					if name, ok := p.Val(); ok && name == nonExistentPolicy {
						t.Fatalf("precondition failed: policy %q unexpectedly exists", nonExistentPolicy)
					}
				}

				t.Log("attempt to apply NON_EXISTENT_POLICY and verify gNMI Set fails")
				bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Update(t, dut, bgpPath.Neighbor(ateP2.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{nonExistentPolicy})
				}); errMsg == nil {
					t.Errorf("error: expected gNMI Set to fail for non-existent policy, but it succeeded")
				} else {
					t.Logf(`gNMI Set error as expected: %v`, *errMsg)
				}

				t.Log("verify BGP remains established and prefix advertisement is unchanged")
				cfgplugins.VerifyDUTBGPEstablished(t, dut)
				cfgplugins.VerifyOTGBGPEstablished(t, ate)

				verifyPrefixes(t, dut, ateP1.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, routeCount, false, true)
				verifyPrefixes(t, dut, ateP1.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, false, true)
				verifyPrefixes(t, dut, ateP2.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, routeCount, true, false)
				verifyPrefixes(t, dut, ateP2.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, routeCount, true, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}
