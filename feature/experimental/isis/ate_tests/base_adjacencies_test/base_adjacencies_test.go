// Copyright 2022 Google LLC
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

package base_adjacencies_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/experimental/isis/ate_tests/internal/assert"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/ate_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/ixnet"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	telemetry "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	PTISIS   = telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	ISISName = "osiris"
)

func maybeUint32(t testing.TB, getter func(t testing.TB) uint32) (got uint32, ok bool) {
	testt.CaptureFatal(t, func(t testing.TB) {
		got, ok = getter(t), true
	})
	return got, ok
}

// TestBasic configures IS-IS on the DUT and confirms that the various values and defaults propagate
// then configures the ATE as well, waits for the adjacency to form, and checks that numerous
// counters and other values now have sensible values.
func TestBasic(t *testing.T) {
	ts := session.NewWithISIS(t)
	// Only push DUT config - no adjacency established yet
	ts.PushDUT(t)
	isisRoot := ts.DUTISISTelemetry(t)
	isisRoot.Global().Instance().Await(t, time.Second, session.ISISName)

	t.Run("read_config", func(t *testing.T) {
		assert.AssertValue(t, isisRoot.Global().Net(), []string{"49.0001.1920.0000.2001.00"})
		assert.AssertValue(t, isisRoot.Global().LevelCapability(), telemetry.IsisTypes_LevelType_LEVEL_1_2)
		assert.AssertValue(t, isisRoot.Global().Af(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Enabled(), true)
		assert.AssertValue(t, isisRoot.Global().Af(telemetry.IsisTypes_AFI_TYPE_IPV6, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Enabled(), true)
		assert.AssertValue(t, isisRoot.Level(2).Enabled(), true)
		assert.AssertValue(t, isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).Enabled(), true)
		assert.AssertValue(t, isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).CircuitType(), telemetry.IsisTypes_CircuitType_POINT_TO_POINT)
	})
	t.Run("read_auth", func(t *testing.T) {
		// TODO: Enable these tests once supported
		t.Skip("Authentication not supported")
		assert.AssertValue(t, isisRoot.Global().AuthenticationCheck(), true)
		l2auth := isisRoot.Level(2).Authentication()
		assert.AssertValue(t, l2auth.DisableCsnp(), false)
		assert.AssertValue(t, l2auth.DisablePsnp(), false)
		assert.AssertValue(t, l2auth.DisableLsp(), false)
	})
	t.Run("counters_before_any_adjacencies", func(t *testing.T) {

		t.Run("packet_counters", func(t *testing.T) {
			pCounts := isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).Level(2).PacketCounters()
			assert.AssertValueOrNil(t, pCounts.Csnp().Dropped(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Csnp().Processed(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Csnp().Received(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Csnp().Sent(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Psnp().Dropped(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Psnp().Processed(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Psnp().Received(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Psnp().Sent(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Lsp().Dropped(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Lsp().Processed(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Lsp().Received(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Lsp().Sent(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Iih().Dropped(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Iih().Processed(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Iih().Received(), uint32(0))
			assert.AssertValueOrNil(t, pCounts.Iih().Sent(), uint32(0))
		})

		t.Run("circuit_counters", func(t *testing.T) {
			cCounts := isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).CircuitCounters()
			assert.AssertValueOrNil(t, cCounts.AdjChanges(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.AdjNumber(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.AuthFails(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.AuthTypeFails(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.IdFieldLenMismatches(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.LanDisChanges(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.MaxAreaAddressMismatches(), uint32(0))
			assert.AssertValueOrNil(t, cCounts.RejectedAdj(), uint32(0))
			if got, ok := maybeUint32(t, cCounts.InitFails().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
		})

		t.Run("level_counters", func(t *testing.T) {
			sysCounts := isisRoot.Level(2).SystemLevelCounters()
			assert.AssertValueOrNil(t, sysCounts.AuthFails(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.AuthTypeFails(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.CorruptedLsps(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.DatabaseOverloads(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.ExceedMaxSeqNums(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.IdLenMismatch(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.LspErrors(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.MaxAreaAddressMismatches(), uint32(0))
			if got, ok := maybeUint32(t, sysCounts.ManualAddressDropFromAreas().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
			assert.AssertValueOrNil(t, sysCounts.OwnLspPurges(), uint32(0))
			assert.AssertValueOrNil(t, sysCounts.SeqNumSkips(), uint32(0))
			if got, ok := maybeUint32(t, sysCounts.PartChanges().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
			assert.AssertValueOrNil(t, sysCounts.SpfRuns(), uint32(1))
		})
	})

	// Form the adjacency
	ts.PushAndStartATE(t)
	ts.AwaitAdjacency(t)

	t.Run("adjacency_state", func(t *testing.T) {
		telem := ts.DUT.Telemetry().NetworkInstance("default").Protocol(PTISIS, ISISName)
		systemID := telem.Isis().Interface(ts.DUT.Port(t, "port1").Name()).Level(2).AdjacencyAny().SystemId().Get(t)
		adj := telem.Isis().Interface(ts.DUT.Port(t, "port1").Name()).Level(2).Adjacency(systemID[0])
		assert.AssertValue(t, adj.AdjacencyState(), telemetry.IsisTypes_IsisInterfaceAdjState_UP)
		assert.AssertValue(t, adj.SystemId(), systemID[0])
		assert.AssertValue(t, adj.AreaAddress(), []string{session.ATEAreaAddress, session.DUTAreaAddress})
		assert.AssertValue(t, adj.DisSystemId(), "0000.0000.0000")
		assert.AssertNonZero(t, adj.LocalExtendedCircuitId())
		assert.AssertValue(t, adj.MultiTopology(), false)
		assert.AssertValue(t, adj.NeighborCircuitType(), telemetry.IsisTypes_LevelType_LEVEL_2)
		assert.AssertNonZero(t, adj.NeighborExtendedCircuitId())
		assert.AssertValue(t, adj.NeighborIpv4Address(), session.ATEISISAttrs.IPv4)
		assert.AssertValue(t, adj.NeighborSnpa(), "00:00:00:00:00:00")
		assert.AssertValue(t, adj.Nlpid(), []telemetry.E_Adjacency_Nlpid{telemetry.Adjacency_Nlpid_IPV4, telemetry.Adjacency_Nlpid_IPV6})
		assert.AssertPresent(t, adj.NeighborIpv6Address())
		assert.AssertPresent(t, adj.Priority())
		assert.AssertPresent(t, adj.RestartStatus())
		assert.AssertPresent(t, adj.RestartSupport())
		assert.AssertPresent(t, adj.RestartSuppress())
	})

	t.Run("counters_after_adjacency", func(t *testing.T) {
		// Wait for at least one CSNP, PSNP, and LSP to have gone by, then confirm the corresponding
		// processed/received/sent counters are nonzero while all the error and dropped counters remain
		// at 0.
		pCounts := isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).Level(2).PacketCounters()
		if _, ok := pCounts.Csnp().Processed().Watch(t, time.Second*5, func(val *telemetry.QualifiedUint32) bool {
			return val != nil && val.IsPresent() && val.Val(t) > uint32(0)
		}).Await(t); !ok {
			t.Errorf("No CSNP messages in active adjacency after 5s.")
		}
		if _, ok := pCounts.Lsp().Processed().Watch(t, time.Second*5, func(val *telemetry.QualifiedUint32) bool {
			return val != nil && val.IsPresent() && val.Val(t) > uint32(0)
		}).Await(t); !ok {
			t.Errorf("No LSP messages in active adjacency after 5s.")
		}
		if _, ok := pCounts.Psnp().Processed().Watch(t, time.Second*5, func(val *telemetry.QualifiedUint32) bool {
			return val != nil && val.IsPresent() && val.Val(t) > uint32(0)
		}).Await(t); !ok {
			t.Errorf("No PSNP messages in active adjacency after 5s.")
		}

		t.Run("packet_counters", func(t *testing.T) {
			pCounts := isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).Level(2).PacketCounters()
			assert.AssertNonZero(t, pCounts.Csnp().Processed())
			assert.AssertNonZero(t, pCounts.Csnp().Received())
			assert.AssertNonZero(t, pCounts.Csnp().Sent())
			assert.AssertNonZero(t, pCounts.Psnp().Processed())
			assert.AssertNonZero(t, pCounts.Psnp().Received())
			assert.AssertNonZero(t, pCounts.Psnp().Sent())
			assert.AssertNonZero(t, pCounts.Lsp().Processed())
			assert.AssertNonZero(t, pCounts.Lsp().Received())
			assert.AssertNonZero(t, pCounts.Lsp().Sent())
			assert.AssertNonZero(t, pCounts.Iih().Processed())
			assert.AssertNonZero(t, pCounts.Iih().Received())
			assert.AssertNonZero(t, pCounts.Iih().Sent())
			// No dropped messages
			assert.AssertValue(t, pCounts.Csnp().Dropped(), uint32(0))
			assert.AssertValue(t, pCounts.Psnp().Dropped(), uint32(0))
			assert.AssertValue(t, pCounts.Lsp().Dropped(), uint32(0))
			assert.AssertValue(t, pCounts.Iih().Dropped(), uint32(0))
		})

		t.Run("circuit_counters", func(t *testing.T) {
			// Only adjChanges and adjNumber should have gone up - others should still be 0
			cCounts := isisRoot.Interface(ts.DUT.Port(t, "port1").Name()).CircuitCounters()
			assert.AssertNonZero(t, cCounts.AdjChanges())
			assert.AssertNonZero(t, cCounts.AdjNumber())
			assert.AssertValue(t, cCounts.AuthFails(), uint32(0))
			assert.AssertValue(t, cCounts.AuthTypeFails(), uint32(0))
			assert.AssertValue(t, cCounts.IdFieldLenMismatches(), uint32(0))
			assert.AssertValue(t, cCounts.LanDisChanges(), uint32(0))
			assert.AssertValue(t, cCounts.MaxAreaAddressMismatches(), uint32(0))
			if got, ok := maybeUint32(t, cCounts.InitFails().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
			assert.AssertValue(t, cCounts.RejectedAdj(), uint32(0))
		})

		t.Run("level_counters", func(t *testing.T) {
			// Error counters should still be zero
			sysCounts := isisRoot.Level(2).SystemLevelCounters()
			assert.AssertValue(t, sysCounts.AuthFails(), uint32(0))
			assert.AssertValue(t, sysCounts.AuthTypeFails(), uint32(0))
			assert.AssertValue(t, sysCounts.CorruptedLsps(), uint32(0))
			assert.AssertValue(t, sysCounts.DatabaseOverloads(), uint32(0))
			assert.AssertValue(t, sysCounts.ExceedMaxSeqNums(), uint32(0))
			assert.AssertValue(t, sysCounts.IdLenMismatch(), uint32(0))
			assert.AssertValue(t, sysCounts.LspErrors(), uint32(0))
			assert.AssertValue(t, sysCounts.MaxAreaAddressMismatches(), uint32(0))
			assert.AssertValue(t, sysCounts.OwnLspPurges(), uint32(0))
			assert.AssertValue(t, sysCounts.SeqNumSkips(), uint32(0))
			if got, ok := maybeUint32(t, sysCounts.ManualAddressDropFromAreas().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
			if got, ok := maybeUint32(t, sysCounts.PartChanges().Get); ok && got != 0 {
				t.Errorf("InitFails got %d, want %d", got, 0)
			}
			assert.AssertValue(t, sysCounts.SpfRuns(), uint32(2))
		})
	})
}

// TestHelloPadding tests several different hello padding modes to confirm they all work.
func TestHelloPadding(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode telemetry.E_IsisTypes_HelloPaddingType
		skip string
	}{
		{
			name: "disabled",
			mode: telemetry.IsisTypes_HelloPaddingType_DISABLE,
		}, {
			name: "strict",
			mode: telemetry.IsisTypes_HelloPaddingType_STRICT,
		}, {
			name: "adaptive",
			mode: telemetry.IsisTypes_HelloPaddingType_ADAPTIVE,
		}, {
			name: "loose",
			mode: telemetry.IsisTypes_HelloPaddingType_LOOSE,
			// TODO: Skip based on deviations.
			skip: "Unsupported",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			ts := session.NewWithISIS(t)
			ts.ConfigISIS(t, func(isis *telemetry.NetworkInstance_Protocol_Isis) {
				global := isis.GetOrCreateGlobal()
				global.HelloPadding = tc.mode
			}, func(isis *ixnet.ISIS) {
				isis.WithHelloPaddingEnabled(tc.mode != telemetry.IsisTypes_HelloPaddingType_DISABLE)
			})
			ts.PushAndStart(t)
			ts.AwaitAdjacency(t)
			telemPth := ts.DUTISISTelemetry(t).Global()
			assert.AssertValue(t, telemPth.HelloPadding(), tc.mode)
		})
	}
}

// TestAuthentication verifies that with authentication enabled or disabled we can still establish
// an IS-IS session with the ATE.
func TestAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    telemetry.E_IsisTypes_AUTH_MODE
		enabled bool
	}{
		{name: "enabled", mode: telemetry.IsisTypes_AUTH_MODE_MD5, enabled: true},
		{name: "enabled", mode: telemetry.IsisTypes_AUTH_MODE_TEXT, enabled: true},
		{name: "disabled", mode: telemetry.IsisTypes_AUTH_MODE_TEXT, enabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := session.NewWithISIS(t)
			ts.ConfigISIS(t, func(isis *telemetry.NetworkInstance_Protocol_Isis) {
				level := isis.GetOrCreateLevel(2)
				level.Enabled = ygot.Bool(true)
				auth := level.GetOrCreateAuthentication()
				auth.Enabled = ygot.Bool(true)
				auth.AuthMode = tc.mode
				auth.AuthType = telemetry.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
				auth.AuthPassword = ygot.String("google")
				for _, intf := range isis.Interface {
					intf.GetOrCreateLevel(2).GetOrCreateHelloAuthentication().Enabled = ygot.Bool(tc.enabled)
					if tc.enabled {
						intf.GetLevel(2).GetHelloAuthentication().AuthPassword = ygot.String("google")
						intf.GetLevel(2).GetHelloAuthentication().AuthMode = tc.mode
						intf.GetLevel(2).GetHelloAuthentication().AuthType = telemetry.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
					}
				}
			}, func(isis *ixnet.ISIS) {
				if tc.enabled {
					isis.WithAuthPassword("google")
				} else {
					isis.WithAuthDisabled()
				}
			})
			ts.PushAndStart(t)
			ts.AwaitAdjacency(t)
		})
	}
}

// TestTraffic has the ATE advertise some routes and verifies that traffic sent to the DUT is routed
// appropriately.
func TestTraffic(t *testing.T) {
	ts := session.NewWithISIS(t)
	targetNetwork := &attrs.Attributes{
		Desc:    "External network (simulated by ATE)",
		IPv4:    "198.51.100.0",
		IPv4Len: 24,
		IPv6:    "2001:db8::198:51:100:0",
		IPv6Len: 112,
	}
	deadNetwork := &attrs.Attributes{
		Desc:    "Unreachable network (traffic to it should blackhole)",
		IPv4:    "203.0.113.0",
		IPv4Len: 24,
		IPv6:    "2001:db8::203:0:113:0",
		IPv6Len: 112,
	}

	ts.ConfigISIS(t, func(isis *telemetry.NetworkInstance_Protocol_Isis) {
		// disable global hello padding on the DUT
		global := isis.GetOrCreateGlobal()
		global.HelloPadding = telemetry.IsisTypes_HelloPaddingType_DISABLE
	}, func(isis *ixnet.ISIS) {
		// disable global hello padding on the ATE
		isis.WithHelloPaddingEnabled(false)
	})

	ate := ts.ATE
	// We generate traffic entering along port2 and destined for port1
	srcIntf := ts.ATEInterface(t, "port2")
	dstIntf := ts.ATEInterface(t, "port1")
	// net is a simulated network containing the addresses specified by targetNetwork
	net := dstIntf.AddNetwork("net")
	net.IPv4().WithAddress(targetNetwork.IPv4CIDR()).WithCount(1)
	net.IPv6().WithAddress(targetNetwork.IPv6CIDR()).WithCount(1)
	net.ISIS().WithIPReachabilityExternal().WithIPReachabilityMetric(10)
	t.Logf("Starting protocols on ATE...")
	ts.PushAndStart(t)
	defer ts.ATETop.StopProtocols(t)
	ts.AwaitAdjacency(t)
	t.Logf("Configuring traffic from ATE through DUT...")
	v4Header := ondatra.NewIPv4Header()
	v4Header.DstAddressRange().WithMin(targetNetwork.IPv4).WithCount(1)
	v4Flow := ate.Traffic().NewFlow("v4Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v4Header)
	v6Header := ondatra.NewIPv6Header()
	v6Header.DstAddressRange().WithMin(targetNetwork.IPv6).WithCount(1)
	v6Flow := ate.Traffic().NewFlow("v6Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v6Header)
	// deadFlow is addressed to a nonexistent network as a consistency check -
	// all traffic should be blackholed.
	deadHeader := ondatra.NewIPv4Header()
	deadHeader.DstAddressRange().WithMin(deadNetwork.IPv4).WithCount(1)
	deadFlow := ate.Traffic().NewFlow("flow2").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), deadHeader)
	t.Logf("Running traffic for 30s...")
	ate.Traffic().Start(t, v4Flow, v6Flow, deadFlow)
	time.Sleep(time.Second * 30)
	ate.Traffic().Stop(t)
	t.Logf("Checking telemetry...")
	telem := ate.Telemetry()
	v4Loss := telem.Flow(v4Flow.Name()).LossPct().Get(t)
	v6Loss := telem.Flow(v6Flow.Name()).LossPct().Get(t)
	deadLoss := telem.Flow(deadFlow.Name()).LossPct().Get(t)
	if v4Loss > 1 {
		t.Errorf("Got %v%% IPv4 packet loss; expected < 1%%", v4Loss)
	}
	if v6Loss > 1 {
		t.Errorf("Got %v%% IPv6 packet loss; expected < 1%%", v6Loss)
	}
	if deadLoss != 100 {
		t.Errorf("Got %v%% invalid packet loss; expected 100%%", deadLoss)
	}
}
