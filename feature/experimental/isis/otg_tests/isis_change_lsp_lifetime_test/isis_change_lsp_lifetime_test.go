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

package isis_change_lsp_lifetime_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
	lspLifetime = 500
	v4Route1    = "203.0.113.0"
	v6Route1    = "2001:db8::203:0:113:0"
	v4Route     = "203.0.113.0/30"
	v6Route     = "2001:db8::203:0:113:0/126"
	v4IP        = "203.0.113.1"
	v6IP        = "2001:db8::203:0:113:1"
	v4NetName   = "isisv4Net"
	v6NetName   = "isisv6Net"
	v4FlowName  = "v4Flow"
	v6FlowName  = "v6Flow"
)

// configureISIS configures isis on DUT.
func configureISIS(t *testing.T, ts *session.TestSession) {
	t.Helper()
	d := ts.DUTConf
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, session.ISISName)
	prot.Enabled = ygot.Bool(true)

	isis := prot.GetOrCreateIsis()
	globalIsis := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(ts.DUT) {
		globalIsis.Instance = ygot.String(session.ISISName)
	}

	// Global configs
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalIsis.GetOrCreateTimers().LspLifetimeInterval = ygot.Uint16(lspLifetime)
	if deviations.ISISLspLifetimeIntervalRequiresLspRefreshInterval(ts.DUT) {
		globalIsis.GetOrCreateTimers().LspRefreshInterval = ygot.Uint16(60)
	}
}

// configureOTG configures isis and traffic on OTG.
func configureOTG(t *testing.T, ts *session.TestSession) {
	t.Helper()

	ts.ATEIntf1.Isis().Advanced().SetLspRefreshRate(60)

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(10)
	netv4.Addresses().Add().SetAddress(v4Route1).SetPrefix(uint32(session.ATEISISAttrs.IPv4Len))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(10)
	netv6.Addresses().Add().SetAddress(v6Route1).SetPrefix(uint32(session.ATEISISAttrs.IPv6Len))

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
	v4Flow.Duration().SetChoice("continuous")
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(session.ATEISISAttrs.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(session.ATEISISAttrs.IPv4)
	v4.Dst().Increment().SetStart(v4IP).SetCount(1)

	t.Log("Configuring v6 traffic flow ")

	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().SetChoice("continuous")
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(session.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(session.ATEISISAttrs.IPv6)
	v6.Dst().Increment().SetStart(v6IP).SetCount(1)
}

// TestISISChangeLSPLifetime verifies isis lsp telemetry paramters with configured lsp lifetime.
func TestISISChangeLSPLifetime(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
	configureISIS(t, ts)

	configureOTG(t, ts)
	otg := ts.ATE.OTG()

	pcl := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, session.ISISName)
	fptest.LogQuery(t, "Protocol ISIS", session.ProtocolPath(ts.DUT).Config(), pcl)

	ts.PushAndStart(t)

	isisPath := session.ISISPath(ts.DUT)
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
		dutLspID := session.DUTSysID + ".00-00"

		// wait for ATE Lsp TLV to be present in DUT
		_, ok := gnmi.Await(t, ts.DUT, isisPath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(v4Route).Prefix().State(), 2*time.Minute, v4Route).Val()
		if !ok {
			t.Error("FAIL- Couldn't find v4Route in ATE LSP TLV")
		}

		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := isisPath.Interface(intfName).Level(2).Adjacency(ateSysID)
			if got := gnmi.Get(t, ts.DUT, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
		})
		t.Run("Lsp checks", func(t *testing.T) {
			isis := gnmi.Get(t, ts.DUT, isisPath.State())
			if got, want := isis.GetGlobal().GetTimers().GetLspLifetimeInterval(), uint16(lspLifetime); got != want {
				t.Errorf("FAIL- Expected lsp lifetime interval not found, got %d, want %d", got, want)
			}
			if got, want := isis.GetLevel(2).GetLsp(dutLspID).GetRemainingLifetime(), uint16(lspLifetime); got >= want {
				t.Errorf("FAIL- Expected remaining lifetime not found, got %d,want less then %d", got, want)
			}
			if got, want := isis.GetLevel(2).GetLsp(dutLspID).GetLspId(), dutLspID; got != want {
				t.Errorf("FAIL- Expected DUT lsp id not found, got %s, want %s", got, want)
			}
			if got, want := isis.GetLevel(2).GetLsp(ateLspID).GetLspId(), ateLspID; got != want {
				t.Errorf("FAIL- Expected ATE lsp not found, got %s, want %s", got, want)
			}
			if got := gnmi.Get(t, ts.DUT, isisPath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State()); got == 0 {
				t.Errorf("FAIL- Expected lsp count is greater than 0, got %d", got)
			}
		})
		t.Run("Route checks", func(t *testing.T) {
			isis := gnmi.Get(t, ts.DUT, isisPath.State())
			if got, want := isis.GetLevel(2).GetLsp(ateLspID).GetTlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).GetExtendedIpv4Reachability().GetPrefix(v4Route).GetPrefix(), v4Route; got != want {
				t.Errorf("FAIL- Expected ate v4 route not found, got %v, want %v", got, want)
			}
			if got, want := isis.GetLevel(2).GetLsp(ateLspID).GetTlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).GetIpv6Reachability().GetPrefix(v6Route).GetPrefix(), v6Route; got != want {
				t.Errorf("FAIL- Expected v6 route not found in isis, got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Afts().Ipv4Entry(v4Route).State()).GetPrefix(), v4Route; got != want {
				t.Errorf("FAIL- Expected v4 route not found in aft, got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, ts.DUT, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).Afts().Ipv6Entry(v6Route).State()).GetPrefix(), v6Route; got != want {
				t.Errorf("FAIL- Expected v6 route not found in aft, got %v, want %v", got, want)
			}
		})
		// Check the lsp's checksum/seq number/remaining lifetime once lsp refreshes periodically.
		t.Run("Lsp lifetime checks", func(t *testing.T) {
			isis := gnmi.Get(t, ts.DUT, isisPath.State())
			seqNum1 := isis.GetLevel(2).GetLsp(dutLspID).GetSequenceNumber()
			checksum1 := isis.GetLevel(2).GetLsp(dutLspID).GetChecksum()
			lspSent1 := gnmi.Get(t, ts.DUT, isisPath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State())

			_, ok := gnmi.Watch(t, ts.DUT, isisPath.Interface(intfName).Level(2).PacketCounters().Lsp().Sent().State(), time.Minute*4, func(val *ygnmi.Value[uint32]) bool {
				lspSent2, ok := val.Val()
				if !ok || lspSent2 <= lspSent1 {
					return false
				}
				if deviations.ISISLspMetadataLeafsUnsupported(ts.DUT) {
					return true
				}
				isisNew := gnmi.Get(t, ts.DUT, isisPath.State())
				if got, want := isisNew.GetLevel(2).GetLsp(dutLspID).GetSequenceNumber(), seqNum1; got <= want {
					t.Errorf("FAIL- Sequence number of new lsp should increment, got %d, want greater than %d", got, want)
				}
				if got := isisNew.GetLevel(2).GetLsp(dutLspID).GetChecksum(); got == checksum1 {
					t.Errorf("FAIL- Checksum of new lsp should be different from %d, got %d", checksum1, got)
				}
				if got := isisNew.GetLevel(2).GetLsp(dutLspID).GetRemainingLifetime(); got >= lspLifetime || got < lspLifetime-50 {
					t.Errorf("FAIL- Expected remaining lifetime not found, got %d,expected b/w %d and %d", got, lspLifetime, lspLifetime-50)
				}
				return true
			}).Await(t)
			if !ok {
				t.Error("FAIL- Isis lsp is not refreshing periodically")
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
