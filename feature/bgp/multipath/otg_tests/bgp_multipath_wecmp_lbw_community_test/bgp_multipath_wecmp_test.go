// Copyright 2023 Google LLC
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

package bgp_multipath_wecmp_test

import (
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	prefixesStart    = "198.51.100.0"
	prefixP4Len      = 32
	prefixesCount    = 3
	pathID           = 1
	maxPaths         = 2
	trafficPps       = 1000
	totalPackets     = 120000
	lossTolerancePct = 0
	lbToleranceFms   = 10
)

var linkBw = []int{10}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession) {
	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)
	for i, otgPort := range bs.ATEPorts {
		if i < 2 {
			continue
		}

		ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
		bgp4Peer := devices[i].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
		bgp4PeerRoute := bgp4Peer.V4Routes().Add()
		bgp4PeerRoute.SetName(otgPort.Name + ".BGP4.peer.rr4")
		bgp4PeerRoute.SetNextHopIpv4Address(ipv4.Address())
		bgp4PeerRoute.SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4)
		bgp4PeerRoute.SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		routeAddress := bgp4PeerRoute.Addresses().Add().SetAddress(prefixesStart)
		routeAddress.SetPrefix(prefixP4Len)
		routeAddress.SetCount(prefixesCount)
		bgp4PeerRoute.AddPath().SetPathId(pathID)
	}

	configureFlow(bs)
}

func attachLBWithInternalNetwork(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	devices := top.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)

	for _, i := range []int{2, 3} {
		bgp4Peer := devices[i].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
		bgp4PeerRoute := bgp4Peer.V4Routes().Items()[0]
		bgp4PeerRoute.ExtendedCommunities().Clear()
		if i-2 < len(linkBw) {
			bgpExtCom := bgp4PeerRoute.ExtendedCommunities().Add()
			bgpExtCom.NonTransitive2OctetAsType().LinkBandwidthSubtype().SetBandwidth(float32(linkBw[i-2] * 1000))
			bgpExtCom.NonTransitive2OctetAsType().LinkBandwidthSubtype().SetGlobal2ByteAs(23456)
		}
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
}

func configureFlow(bs *cfgplugins.BGPSession) {
	bs.ATETop.Flows().Clear()

	var rxNames []string
	for i := 2; i < len(bs.ATEPorts); i++ {
		rxNames = append(rxNames, bs.ATEPorts[i].Name+".BGP4.peer.rr4")
	}
	flow := bs.ATETop.Flows().Add().SetName("flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv4"}).
		SetRxNames(rxNames)
	flow.Duration().FixedPackets().SetPackets(totalPackets)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(trafficPps)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(bs.ATEPorts[0].MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().Increment().SetCount(1000).SetStep("0.0.0.1").SetStart(bs.ATEPorts[0].IPv4)
	v4.Dst().Increment().SetCount(3).SetStep("0.0.0.1").SetStart(prefixesStart)
}

func checkPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	countersPath := gnmi.OTG().Flow("flow").Counters()
	rxPackets := gnmi.Get(t, ate.OTG(), countersPath.InPkts().State())
	txPackets := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())
	lostPackets := txPackets - rxPackets

	if txPackets < 1 {
		t.Fatalf("Tx packets should be higher than 0")
	}
	if got := lostPackets * 100 / txPackets; got != lossTolerancePct {
		t.Errorf("Packet loss percentage for flow: got %v, want %v", got, lossTolerancePct)
	}
}

func verifyECMPLoadBalance(t *testing.T, ate *ondatra.ATEDevice, pc int, expectedLinks int) {
	framesTx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	var lb1, lb2 float32
	if len(linkBw) == 1 {
		lb1, lb2 = float32(linkBw[0]), float32(linkBw[0])
	} else {
		lb1, lb2 = float32(linkBw[0]), float32(linkBw[1])
	}

	expectedPerLinkFmsP3 := int(lb1 / (lb1 + lb2) * float32(framesTx))
	expectedPerLinkFmsP4 := int(lb2 / (lb1 + lb2) * float32(framesTx))
	t.Logf("Total packets %d flow through the %d links and expected per link packets: %d, %d", framesTx, expectedLinks, expectedPerLinkFmsP3, expectedPerLinkFmsP4)

	p3Min := expectedPerLinkFmsP3 - (expectedPerLinkFmsP3 * lbToleranceFms / 100)
	p3Max := expectedPerLinkFmsP3 + (expectedPerLinkFmsP3 * lbToleranceFms / 100)
	framesRxP3 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port"+strconv.Itoa(3)).ID()).Counters().InFrames().State())

	if int64(p3Min) < int64(framesRxP3) && int64(framesRxP3) < int64(p3Max) {
		t.Logf("Traffic of %d on port-3 is within expected range: %d - %d", framesRxP3, p3Min, p3Max)
	} else {
		t.Errorf("Traffic on port-3 is expected to be in the range %d - %d but got %d. Load balance Test Failed", p3Min, p3Max, framesRxP3)
	}

	p4Min := expectedPerLinkFmsP4 - (expectedPerLinkFmsP4 * lbToleranceFms / 100)
	p4Max := expectedPerLinkFmsP4 + (expectedPerLinkFmsP4 * lbToleranceFms / 100)
	framesRxP4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port"+strconv.Itoa(4)).ID()).Counters().InFrames().State())

	if int64(p4Min) < int64(framesRxP4) && int64(framesRxP4) < int64(p4Max) {
		t.Logf("Traffic of %d on port-4 is within expected range: %d - %d", framesRxP4, p4Min, p4Max)
	} else {
		t.Errorf("Traffic on port-4 is expected to be in the range %d - %d but got %d. Load balance Test Failed", p4Min, p4Max, framesRxP4)
	}
}

func TestBGPSetup(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}, []string{"port3", "port4"}, true, false)

	dni := deviations.DefaultNetworkInstance(bs.DUT)
	bgp := bs.DUTConf.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	switch bs.DUT.Vendor() {
	case ondatra.NOKIA:
		//BGP multipath enable/disable at the peer-group level not required b/376799583
		t.Logf("BGP Multipath enable/disable is not required under Peer-group by %s hence skipping", bs.DUT.Vendor())
	default:
		bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
	}

	if !deviations.SkipBgpSendCommunityType(bs.DUT) {
		bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED, oc.Bgp_CommunityType_LARGE})
	}

	if deviations.MultipathUnsupportedNeighborOrAfisafi(bs.DUT) {
		t.Logf("MultipathUnsupportedNeighborOrAfisafi is supported")
		bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
		bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().AllowMultipleAs = ygot.Bool(true)
	}

	if deviations.SkipAfiSafiPathForBgpMultipleAs(bs.DUT) {
		var communitySetCLIConfig string
		t.Log("AfiSafi Path For BgpMultipleAs is not supported")
		gEBGP := bgp.GetOrCreateGlobal().GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
		gEBGPMP := bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
		gEBGPMP.MaximumPaths = ygot.Uint32(maxPaths)
		if deviations.SkipSettingAllowMultipleAS(bs.DUT) {
			gEBGP.AllowMultipleAs = ygot.Bool(false)
			switch bs.DUT.Vendor() {
			case ondatra.CISCO:
				communitySetCLIConfig = fmt.Sprintf("router bgp %v instance BGP neighbor-group %v \n ebgp-recv-extcommunity-dmz \n ebgp-send-extcommunity-dmz\n", cfgplugins.DutAS, cfgplugins.BGPPeerGroup1)
			default:
				t.Fatalf("Unsupported vendor %s for deviation 'CommunityMemberRegexUnsupported'", bs.DUT.Vendor())
			}
			helpers.GnmiCLIConfig(t, bs.DUT, communitySetCLIConfig)
		}
	} else if deviations.SkipSettingAllowMultipleAS(bs.DUT) {
		bgp.GetOrCreateGlobal().GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(maxPaths)
		bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().GetOrCreateLinkBandwidthExtCommunity().SetEnabled(true)
		switch bs.DUT.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, bs.DUT, "router bgp 65501\n ucmp mode 1\n")
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'SkipSettingAllowMultipleAS'", bs.DUT.Vendor())
		}
	} else {
		gEBGP := bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
		gEBGP.SetAllowMultipleAs(true)
		gEBGP.SetMaximumPaths(maxPaths)
		gEBGP.GetOrCreateLinkBandwidthExtCommunity().SetEnabled(true)
	}

	configureOTG(t, bs)

	t.Logf("Waiting for all DUT ports to be operationally UP")
	for _, p := range bs.OndatraDUTPorts {
		gnmi.Await(t, bs.DUT, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
	}

	bs.PushAndStart(t)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

	testCases := []struct {
		name   string
		desc   string
		linkBw []int
	}{
		{
			name:   "RT-1.52.1",
			desc:   "Verify BGP multipath when some path missing link-bandwidth extended-community",
			linkBw: []int{10},
		},
		{
			name:   "RT-1.52.2",
			desc:   "Verify use of equal community type",
			linkBw: []int{10, 10},
		},
		{
			name:   "RT-1.52.1",
			desc:   "Verify use of unequal community type",
			linkBw: []int{10, 5},
		},
	}

	aftsPath := gnmi.OC().NetworkInstance(dni).Afts()
	prefix := prefixesStart + "/" + strconv.Itoa(prefixP4Len)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			linkBw = tc.linkBw
			attachLBWithInternalNetwork(t, bs.ATE, bs.ATETop)
			time.Sleep(30 * time.Second)

			ipv4Entry := gnmi.Get[*oc.NetworkInstance_Afts_Ipv4Entry](t, bs.DUT, aftsPath.Ipv4Entry(prefix).State())
			hopGroup := gnmi.Get[*oc.NetworkInstance_Afts_NextHopGroup](t, bs.DUT, aftsPath.NextHopGroup(ipv4Entry.GetNextHopGroup()).State())
			if got, want := len(hopGroup.NextHop), 2; got != want {
				t.Errorf("prefix: %s, found %d hops, want %d", ipv4Entry.GetPrefix(), got, want)
			} else {
				for i, nh := range hopGroup.NextHop {
					t.Logf("Prefix %s, NextHop(%d) weight: %d", ipv4Entry.GetPrefix(), i, nh.GetWeight())
				}
			}

			sleepTime := time.Duration(totalPackets/trafficPps) + 10
			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)

			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			checkPacketLoss(t, bs.ATE)
			verifyECMPLoadBalance(t, bs.ATE, int(cfgplugins.PortCount4), 2)
		})
	}
}
