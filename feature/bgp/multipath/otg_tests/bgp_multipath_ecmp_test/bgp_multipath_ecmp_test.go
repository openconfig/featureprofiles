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

package bgp_multipath_ecmp_test

import (
	"sort"
	"strconv"
	"testing"
	"time"

	"math/rand"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	prefixesStart    = "198.51.100.0"
	prefixP4Len      = 32
	prefixesCount    = 4
	pathID           = 1
	maxPaths         = 2
	trafficPps       = 100000
	totalPackets     = 12000000
	lossTolerancePct = 0
	lbToleranceFms   = 20
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession) {
	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)
	for i, otgPort := range bs.ATEPorts {
		if i == 0 {
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

	configureFlow(t, bs)
}

func randRange(t *testing.T, start, end uint32, count int) []uint32 {
	if count > int(end-start) {
		t.Fatal("randRange: count greater than end-start.")
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		diff := end - start
		randomValue := rand.Int31n(int32(diff)) + int32(start)
		result = append(result, uint32(randomValue))
	}
	return result
}

func configureFlow(t *testing.T, bs *cfgplugins.BGPSession) {
	bs.ATETop.Flows().Clear()

	var rxNames []string
	for i := 1; i < len(bs.ATEPorts); i++ {
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
	v4.Dst().Increment().SetCount(4).SetStep("0.0.0.1").SetStart(prefixesStart)
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(t, 34525, 65535, 500))
	udp.DstPort().SetValues(randRange(t, 49152, 65535, 500))
	v4.Src().SetValue(bs.ATEPorts[0].IPv4)
	v4.Dst().SetValue(prefixesStart)
}

func verifyECMPLoadBalance(t *testing.T, ate *ondatra.ATEDevice, pc int, expectedLinks int) {
	dut := ondatra.DUT(t, "dut")
	framesTx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	expectedPerLinkFms := framesTx / uint64(expectedLinks)
	t.Logf("Total packets %d flow through the %d links and expected per link packets %d", framesTx, expectedLinks, expectedPerLinkFms)
	min := expectedPerLinkFms - (expectedPerLinkFms * lbToleranceFms / 100)
	max := expectedPerLinkFms + (expectedPerLinkFms * lbToleranceFms / 100)

	got := 0
	for i := 2; i <= pc; i++ {
		framesRx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port"+strconv.Itoa(i)).ID()).Counters().InFrames().State())
		if framesRx <= lbToleranceFms {
			t.Logf("Skip: Traffic through port%d interface is %d", i, framesRx)
			continue
		}
		if int64(min) < int64(framesRx) && int64(framesRx) < int64(max) {
			t.Logf("Traffic %d is in expected range: %d - %d, Load balance Test Passed", framesRx, min, max)
			got++
		} else {
			if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
				t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed", min, max, framesRx)
			}
		}
	}
	if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
		if got != expectedLinks {
			t.Errorf("invalid number of load balancing interfaces, got: %d want %d", got, expectedLinks)
		}
	}
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

type testCase struct {
	desc            string
	enableMultipath bool
	enableMultiAS   bool
	expectedPaths   int
}

func TestBGPSetup(t *testing.T) {
	testCases := []testCase{
		{
			desc:            "ebgp setup test",
			enableMultipath: false,
			enableMultiAS:   false,
			expectedPaths:   1,
		},
		{
			desc:            "ebgp multipath same as",
			enableMultipath: true,
			enableMultiAS:   false,
			expectedPaths:   2,
		},
		{
			desc:            "ebgp multipath different as",
			enableMultipath: true,
			enableMultiAS:   true,
			expectedPaths:   2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
			bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}, []string{"port2", "port3", "port4"}, true, !tc.enableMultiAS)
			dni := deviations.DefaultNetworkInstance(bs.DUT)
			bgp := bs.DUTConf.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
			gEBGP := bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
			pgUseMulitplePaths := bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths()
			if tc.enableMultipath {
				t.Logf("Enable Multipath")
				pgUseMulitplePaths.Enabled = ygot.Bool(true)
				t.Logf("Enable Maximum Paths")
				gEBGP.MaximumPaths = ygot.Uint32(maxPaths)
			}
			if tc.enableMultiAS && !deviations.SkipSettingAllowMultipleAS(bs.DUT) && deviations.SkipAfiSafiPathForBgpMultipleAs(bs.DUT) {
				t.Logf("Enable MultiAS ")
				gEBGP := bgp.GetOrCreateGlobal().GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
				gEBGP.AllowMultipleAs = ygot.Bool(true)
			}
			if tc.enableMultiAS && !deviations.SkipSettingAllowMultipleAS(bs.DUT) && !deviations.SkipAfiSafiPathForBgpMultipleAs(bs.DUT) {
				t.Logf("Enable MultiAS ")
				gEBGP.AllowMultipleAs = ygot.Bool(true)
			}

			configureOTG(t, bs)
			bs.PushAndStart(t)

			t.Logf("Verify DUT BGP sessions up")
			cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)

			t.Logf("Verify OTG BGP sessions up")
			cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

			aftsPath := gnmi.OC().NetworkInstance(dni).Afts()
			prefix := prefixesStart + "/" + strconv.Itoa(prefixP4Len)
			ipv4Entry := gnmi.Get[*oc.NetworkInstance_Afts_Ipv4Entry](t, bs.DUT, aftsPath.Ipv4Entry(prefix).State())
			hopGroup := gnmi.Get[*oc.NetworkInstance_Afts_NextHopGroup](t, bs.DUT, aftsPath.NextHopGroup(ipv4Entry.GetNextHopGroup()).State())
			if deviations.BgpMaxMultipathPathsUnsupported(bs.DUT) {
				tc.expectedPaths = 3
			} else {
				if got, want := len(hopGroup.NextHop), tc.expectedPaths; got != want {
					t.Errorf("prefix: %s, found %d hops, want %d", ipv4Entry.GetPrefix(), got, want)
				}
			}

			sleepTime := time.Duration(totalPackets/trafficPps) + 5
			bs.ATE.OTG().StartTraffic(t)
			time.Sleep(sleepTime * time.Second)
			bs.ATE.OTG().StopTraffic(t)

			otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)
			checkPacketLoss(t, bs.ATE)
			verifyECMPLoadBalance(t, bs.ATE, int(cfgplugins.PortCount4), tc.expectedPaths)
		})
	}
}
