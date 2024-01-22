// Copyright 2023 Google LLC
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

package isis_interface_hello_padding_enable_test

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plenIPv4   = 30
	plenIPv6   = 126
	password   = "google"
	v4Route1   = "203.0.113.0"
	v6Route1   = "2001:db8::203:0:113:0"
	v4Route    = "203.0.113.0/30"
	v6Route    = "2001:db8::203:0:113:0/126"
	v4IP       = "203.0.113.1"
	v6IP       = "2001:db8::203:0:113:1"
	v4Metric   = 100
	v6Metric   = 100
	v4NetName  = "isisv4Net"
	v6NetName  = "isisv6Net"
	v4FlowName = "v4Flow"
	v6FlowName = "v6Flow"
)

// configureISIS configures isis on DUT.
func configureISIS(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	d := ts.DUTConf
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	prot.Enabled = ygot.Bool(true)

	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	// Global configs.
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	globalISIS.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

	// Level configs.
	level := isis.GetOrCreateLevel(2)
	level.LevelNumber = ygot.Uint8(2)

	// Authentication configs.
	auth := level.GetOrCreateAuthentication()
	auth.Enabled = ygot.Bool(true)
	auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	auth.AuthPassword = ygot.String(password)

	// Interface configs.
	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	intf := isis.GetOrCreateInterface(intfName)
	intf.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

	// Interface timers.
	isisIntfTimers := intf.GetOrCreateTimers()
	isisIntfTimers.CsnpInterval = ygot.Uint16(5)
	if deviations.ISISTimersCsnpIntervalUnsupported(ts.DUT) {
		isisIntfTimers.CsnpInterval = nil
	}
	isisIntfTimers.LspPacingInterval = ygot.Uint64(150)

	// Interface level configs.
	isisIntfLevel := intf.GetOrCreateLevel(2)
	isisIntfLevel.LevelNumber = ygot.Uint8(2)
	isisIntfLevel.GetOrCreateHelloAuthentication().Enabled = ygot.Bool(true)
	isisIntfLevel.GetHelloAuthentication().AuthPassword = ygot.String(password)
	isisIntfLevel.GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	isisIntfLevel.GetHelloAuthentication().AuthMode = oc.IsisTypes_AUTH_MODE_MD5

	isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
	isisIntfLevelTimers.HelloInterval = ygot.Uint32(5)
	isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(3)

	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v4Metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v6Metric)
}

// configureOTG configures isis and traffic on OTG.
func configureOTG(t *testing.T, ts *isissession.TestSession) {
	t.Helper()

	ts.ATEIntf1.Isis().RouterAuth().AreaAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().RouterAuth().DomainAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().Basic().SetEnableWideMetric(true).SetLearnedLspFilter(false)
	ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetAuthType("md5").SetMd5(password)

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(10)
	netv4.Addresses().Add().SetAddress(v4Route1).SetPrefix(uint32(isissession.ATEISISAttrs.IPv4Len))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(10)
	netv6.Addresses().Add().SetAddress(v6Route1).SetPrefix(uint32(isissession.ATEISISAttrs.IPv6Len))

	// We generate traffic entering along port2 and destined for port1
	srcIpv4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcIpv6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	t.Log("Configuring v4 traffic flow ")

	v4Flow := ts.ATETop.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{v4NetName})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().Continuous()
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(isissession.ATEISISAttrs.IPv4)
	v4.Dst().Increment().SetStart(v4IP).SetCount(1)

	t.Log("Configuring v6 traffic flow ")

	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().Continuous()
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(isissession.ATEISISAttrs.IPv6)
	v6.Dst().Increment().SetStart(v6IP).SetCount(1)
}

// TestIsisInterfaceHelloPaddingEnable verifies adjacency with hello padding enabled.
func TestIsisInterfaceHelloPaddingEnable(t *testing.T) {
	isissession.DUTISISAttrs.MTU = 1500
	isissession.ATEISISAttrs.MTU = 1500

	ts := isissession.MustNew(t).WithISIS()
	configureISIS(t, ts)

	configureOTG(t, ts)
	otg := ts.ATE.OTG()

	pcl := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	fptest.LogQuery(t, "Protocol ISIS", isissession.ProtocolPath(ts.DUT).Config(), pcl)

	ts.PushAndStart(t)

	statePath := isissession.ISISPath(ts.DUT)
	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	t.Run("Isis telemetry", func(t *testing.T) {

		// Checking adjacency
		ateSysID, err := ts.AwaitAdjacency()
		if err != nil {
			t.Fatalf("Adjacency state invalid: %v", err)
		}
		ateLspID := ateSysID + ".00-00"

		t.Run("HelloPadding checks", func(t *testing.T) {
			if got := gnmi.Get(t, ts.DUT, statePath.Global().HelloPadding().State()); got != oc.Isis_HelloPaddingType_ADAPTIVE {
				t.Errorf("FAIL- Expected global hello padding state not found, got %d, want %d", got, oc.Isis_HelloPaddingType_ADAPTIVE)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).HelloPadding().State()); got != oc.Isis_HelloPaddingType_ADAPTIVE {
				t.Errorf("FAIL- Expected interface hello padding state not found, got %d, want %d", got, oc.Isis_HelloPaddingType_ADAPTIVE)
			}
			// Changing MTU at ATE side.
			for _, d := range ts.ATETop.Devices().Items() {
				Eth := d.Ethernets().Items()[0]
				if Eth.Name() == isissession.ATEISISAttrs.Name+".Eth" {
					Eth.SetMtu(uint32(2000))
				}
			}
			otg.PushConfig(t, ts.ATETop)
			otg.StartProtocols(t)

			// Adjacency check.
			_, found := gnmi.Watch(t, ts.DUT, statePath.Interface(intfName).Level(2).Adjacency(ateSysID).AdjacencyState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_DOWN
			}).Await(t)
			if !found {
				t.Errorf("Isis adjacency is not down on interface %v when MTU is changed", intfName)
			}

			// Reverting MTU at ATE side.
			for _, d := range ts.ATETop.Devices().Items() {
				Eth := d.Ethernets().Items()[0]
				if Eth.Name() == isissession.ATEISISAttrs.Name+".Eth" {
					Eth.SetMtu(uint32(isissession.ATEISISAttrs.MTU))
				}
			}
			otg.PushConfig(t, ts.ATETop)
			otg.StartProtocols(t)

			// Adjacency check.
			_, err := ts.AwaitAdjacency()
			if err != nil {
				t.Fatalf("Adjacency should be up: %v", err)
			}
		})
		t.Run("Afi-Safi checks", func(t *testing.T) {
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV4 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV4)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV6 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV6)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v4Metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found, got %d, want %d", got, v4Metric)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v6Metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found, got %d, want %d", got, v6Metric)
			}
		})
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)

			if got := gnmi.Get(t, ts.DUT, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found, got %s, want %s", got, ateSysID)
			}
			want := []string{isissession.ATEAreaAddress, isissession.DUTAreaAddress}
			if got := gnmi.Get(t, ts.DUT, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found, got %s, want %s", got, want)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.DisSystemId().State()); got != "0000.0000.0000" {
				t.Errorf("FAIL- Expected dis system id not found, got %s, want %s", got, "0000.0000.0000")
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value, got %d", got)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found, got %t, want %t", got, false)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found, got %s, want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv4Address().State()); got != isissession.ATEISISAttrs.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found, got %s, want %s", got, isissession.ATEISISAttrs.IPv4)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value, got %d", got)
			}
			snpaAddress := gnmi.Get(t, ts.DUT, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpaAddress)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found, got %s", snpaAddress)
			}
			if got := gnmi.Get(t, ts.DUT, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			ipv6Address := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6Address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found, got %s", ipv6Address)
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.Priority().State()).Val(); !ok {
				t.Errorf("FAIL- Priority is not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart status not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartSupport().State()).Val(); !ok {
				t.Errorf("FAIL- Restart support not present")
			}
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
		t.Run("System level counter checks", func(t *testing.T) {
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().AuthFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication key failure, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().AuthTypeFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication type mismatches, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().CorruptedLsps().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any corrupted lsps, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().DatabaseOverloads().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero database_overloads, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().ExceedMaxSeqNums().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero max_seqnum counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().IdLenMismatch().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero IdLen_Mismatch counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().LspErrors().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any lsp errors, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().MaxAreaAddressMismatches().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero MaxAreaAddressMismatches counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().OwnLspPurges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero OwnLspPurges counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().SeqNumSkips().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero SeqNumber skips, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().ManualAddressDropFromAreas().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero ManualAddressDropFromAreas counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().PartChanges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting partition changes, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().SpfRuns().State()); got == 0 {
				t.Errorf("FAIL- Not expecting spf runs counter to be 0, got %d, want non zero", got)
			}
		})
		t.Run("Route checks", func(t *testing.T) {
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(v4Route).Prefix().State()); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in isis, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(v6Route).Prefix().State()); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in isis, got %v, want %v", got, v6Route)
			}
			if got := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Afts().Ipv4Entry(v4Route).State()).GetPrefix(); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in aft, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.WithISIS().DUT)).Afts().Ipv6Entry(v6Route).State()).GetPrefix(); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in aft, got %v, want %v", got, v6Route)
			}
		})
		t.Run("Traffic checks", func(t *testing.T) {
			t.Logf("Starting traffic")
			otg.StartTraffic(t)
			time.Sleep(time.Second * 15)
			t.Logf("Stop traffic")
			otg.StopTraffic(t)

			otgutils.LogFlowMetrics(t, otg, ts.ATETop)
			otgutils.LogPortMetrics(t, otg, ts.ATETop)

			for _, flow := range []string{v4FlowName, v6FlowName} {
				t.Log("Checking flow telemetry...")
				recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
				txPackets := recvMetric.GetCounters().GetOutPkts()
				rxPackets := recvMetric.GetCounters().GetInPkts()
				lostPackets := txPackets - rxPackets
				lossPct := lostPackets * 100 / txPackets

				if lossPct > 1 {
					t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flow)
				}
			}
		})
	})
}
