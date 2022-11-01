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
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/check"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/ixnet"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// EqualToDefault is the same as check.Equal unless the AllowNilForDefaults
// deviation is set, in which case it uses check.EqualOrNil to allow the device
// to return a nil value. This should only be used when `val` is the default
// for this particular query.
func EqualToDefault[T any](query ygnmi.SingletonQuery[T], val T) check.Validator {
	if *deviations.MissingValueForDefaults {
		return check.EqualOrNil(query, val)
	}
	return check.Equal(query, val)
}

// TestBasic configures IS-IS on the DUT and confirms that the various values and defaults propagate
// then configures the ATE as well, waits for the adjacency to form, and checks that numerous
// counters and other values now have sensible values.
func TestBasic(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
	// Only push DUT config - no adjacency established yet
	if err := ts.PushDUT(context.Background()); err != nil {
		t.Fatalf("Unable to push initial DUT config: %v", err)
	}
	isisRoot := session.ISISPath()
	port1ISIS := isisRoot.Interface(ts.DUTPort1.Name())
	if err := check.Equal(isisRoot.Global().Instance().State(), session.ISISName).AwaitFor(time.Second, ts.DUTClient); err != nil {
		t.Fatalf("IS-IS failed to configure: %v", err)
	}
	// There might be lag between when the instance name is set and when the
	// other parameters are set; we expect the total lag to be under 5s
	deadline := time.Now().Add(time.Second * 5)

	t.Run("read_config", func(t *testing.T) {
		for _, vd := range []check.Validator{
			check.Equal(isisRoot.Global().Net().State(), []string{"49.0001.1920.0000.2001.00"}),
			EqualToDefault(isisRoot.Global().LevelCapability().State(), oc.Isis_LevelType_LEVEL_1_2),
			check.Equal(isisRoot.Global().Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true),
			check.Equal(isisRoot.Global().Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true),
			check.Equal(isisRoot.Level(2).Enabled().State(), true),
			check.Equal(port1ISIS.Enabled().State(), true),
			check.Equal(port1ISIS.CircuitType().State(), oc.Isis_CircuitType_POINT_TO_POINT),
		} {
			t.Run(vd.RelPath(isisRoot), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Error(err)
				}
			})
		}
	})
	t.Run("read_auth", func(t *testing.T) {
		// TODO: Enable these tests once supported
		t.Skip("Authentication not supported")
		l2auth := isisRoot.Level(2).Authentication()
		for _, vd := range []check.Validator{
			check.Equal(isisRoot.Global().AuthenticationCheck().State(), true),
			check.Equal(l2auth.DisableCsnp().State(), false),
			check.Equal(l2auth.DisablePsnp().State(), false),
			check.Equal(l2auth.DisableLsp().State(), false),
		} {
			t.Run(vd.RelPath(isisRoot), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Error(err)
				}
			})
		}
	})
	var spfBefore uint32
	t.Run("counters_before_any_adjacencies", func(t *testing.T) {
		if val, err := ygnmi.Lookup(context.Background(), ts.DUTClient, isisRoot.Level(2).SystemLevelCounters().SpfRuns().State()); err != nil {
			t.Errorf("Unable to read spf run counter before adjancencies: %v", err)
		} else {
			v, present := val.Val()
			if present {
				spfBefore = v
			}
		}

		t.Run("packet_counters", func(t *testing.T) {
			pCounts := port1ISIS.Level(2).PacketCounters()
			for _, vd := range []check.Validator{
				EqualToDefault(pCounts.Csnp().Dropped().State(), uint32(0)),
				EqualToDefault(pCounts.Csnp().Processed().State(), uint32(0)),
				EqualToDefault(pCounts.Csnp().Received().State(), uint32(0)),
				EqualToDefault(pCounts.Csnp().Sent().State(), uint32(0)),
				EqualToDefault(pCounts.Psnp().Dropped().State(), uint32(0)),
				EqualToDefault(pCounts.Psnp().Processed().State(), uint32(0)),
				EqualToDefault(pCounts.Psnp().Received().State(), uint32(0)),
				EqualToDefault(pCounts.Psnp().Sent().State(), uint32(0)),
				EqualToDefault(pCounts.Lsp().Dropped().State(), uint32(0)),
				EqualToDefault(pCounts.Lsp().Processed().State(), uint32(0)),
				EqualToDefault(pCounts.Lsp().Received().State(), uint32(0)),
				EqualToDefault(pCounts.Lsp().Sent().State(), uint32(0)),
				EqualToDefault(pCounts.Iih().Dropped().State(), uint32(0)),
				EqualToDefault(pCounts.Iih().Processed().State(), uint32(0)),
				EqualToDefault(pCounts.Iih().Received().State(), uint32(0)),
				// Don't check IIH sent - the device can send hellos even if the other
				// end is offline.
			} {
				t.Run(vd.RelPath(pCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})

		t.Run("circuit_counters", func(t *testing.T) {
			cCounts := port1ISIS.CircuitCounters()
			for _, vd := range []check.Validator{
				EqualToDefault(cCounts.AdjChanges().State(), uint32(0)),
				EqualToDefault(cCounts.AdjNumber().State(), uint32(0)),
				EqualToDefault(cCounts.AuthFails().State(), uint32(0)),
				EqualToDefault(cCounts.AuthTypeFails().State(), uint32(0)),
				EqualToDefault(cCounts.IdFieldLenMismatches().State(), uint32(0)),
				EqualToDefault(cCounts.LanDisChanges().State(), uint32(0)),
				EqualToDefault(cCounts.MaxAreaAddressMismatches().State(), uint32(0)),
				EqualToDefault(cCounts.RejectedAdj().State(), uint32(0)),
			} {
				t.Run(vd.RelPath(cCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})
		t.Run("level_counters", func(t *testing.T) {
			sysCounts := isisRoot.Level(2).SystemLevelCounters()
			for _, vd := range []check.Validator{
				EqualToDefault(sysCounts.AuthFails().State(), uint32(0)),
				EqualToDefault(sysCounts.AuthTypeFails().State(), uint32(0)),
				EqualToDefault(sysCounts.CorruptedLsps().State(), uint32(0)),
				EqualToDefault(sysCounts.DatabaseOverloads().State(), uint32(0)),
				EqualToDefault(sysCounts.ExceedMaxSeqNums().State(), uint32(0)),
				EqualToDefault(sysCounts.IdLenMismatch().State(), uint32(0)),
				EqualToDefault(sysCounts.LspErrors().State(), uint32(0)),
				EqualToDefault(sysCounts.MaxAreaAddressMismatches().State(), uint32(0)),
				EqualToDefault(sysCounts.OwnLspPurges().State(), uint32(0)),
				EqualToDefault(sysCounts.SeqNumSkips().State(), uint32(0)),
			} {
				t.Run(vd.RelPath(sysCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})
	})

	// Form the adjacency
	ts.PushAndStartATE(t)
	systemID, err := ts.AwaitAdjacency()
	if err != nil {
		t.Fatalf("No IS-IS adjacency formed: %v", err)
	}
	// Allow 1s of lag between adjacency appearing and all data being populated

	t.Run("adjacency_state", func(t *testing.T) {
		deadline = time.Now().Add(time.Second)
		adj := port1ISIS.Level(2).Adjacency(systemID)
		for _, vd := range []check.Validator{
			check.Equal(adj.AdjacencyState().State(), oc.Isis_IsisInterfaceAdjState_UP),
			check.Equal(adj.SystemId().State(), systemID),
			check.Equal(adj.AreaAddress().State(), []string{session.ATEAreaAddress, session.DUTAreaAddress}),
			check.Equal(adj.DisSystemId().State(), "0000.0000.0000"),
			check.NotEqual(adj.LocalExtendedCircuitId().State(), uint32(0)),
			check.Equal(adj.MultiTopology().State(), false),
			check.Equal(adj.NeighborCircuitType().State(), oc.Isis_LevelType_LEVEL_2),
			check.NotEqual(adj.NeighborExtendedCircuitId().State(), uint32(0)),
			check.Equal(adj.NeighborIpv4Address().State(), session.ATEISISAttrs.IPv4),
			check.Equal(adj.NeighborSnpa().State(), "00:00:00:00:00:00"),
			check.Equal(adj.Nlpid().State(), []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}),
			check.Predicate(adj.NeighborIpv6Address().State(), "want a valid IPv6 address", func(got string) bool {
				ip := net.ParseIP(got)
				return ip != nil && ip.To16() != nil
			}),
			check.Present[uint8](adj.Priority().State()),
			check.Present[bool](adj.RestartStatus().State()),
			check.Present[bool](adj.RestartSupport().State()),
			check.Present[bool](adj.RestartSuppress().State()),
		} {
			t.Run(vd.RelPath(adj), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Error(err)
				}
			})
		}
	})

	t.Run("counters_after_adjacency", func(t *testing.T) {
		// Wait for at least one CSNP, PSNP, and LSP to have gone by, then confirm
		// the corresponding processed/received/sent counters are nonzero while all
		// the error and dropped counters remain at 0.
		pCounts := port1ISIS.Level(2).PacketCounters()

		// Note: This is not a subtest because a failure here means checking the
		//   rest of the counters is pointless - none of them will change if we
		//   haven't been exchanging IS-IS messages.
		deadline = time.Now().Add(time.Second * 5)
		for _, vd := range []check.Validator{
			check.NotEqual(pCounts.Csnp().Processed().State(), uint32(0)),
			check.NotEqual(pCounts.Lsp().Processed().State(), uint32(0)),
			check.NotEqual(pCounts.Psnp().Processed().State(), uint32(0)),
		} {
			t.Run(vd.RelPath(pCounts), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Fatalf("No messages in active adjacency after 5s: %v", err)
				}
			})
		}
		deadline = time.Now().Add(time.Second)
		t.Run("packet_counters", func(t *testing.T) {
			pCounts := port1ISIS.Level(2).PacketCounters()
			for _, vd := range []check.Validator{
				check.NotEqual(pCounts.Csnp().Processed().State(), uint32(0)),
				check.NotEqual(pCounts.Csnp().Received().State(), uint32(0)),
				check.NotEqual(pCounts.Csnp().Sent().State(), uint32(0)),
				check.NotEqual(pCounts.Psnp().Processed().State(), uint32(0)),
				check.NotEqual(pCounts.Psnp().Received().State(), uint32(0)),
				check.NotEqual(pCounts.Psnp().Sent().State(), uint32(0)),
				check.NotEqual(pCounts.Lsp().Processed().State(), uint32(0)),
				check.NotEqual(pCounts.Lsp().Received().State(), uint32(0)),
				check.NotEqual(pCounts.Lsp().Sent().State(), uint32(0)),
				check.NotEqual(pCounts.Iih().Processed().State(), uint32(0)),
				check.NotEqual(pCounts.Iih().Received().State(), uint32(0)),
				check.NotEqual(pCounts.Iih().Sent().State(), uint32(0)),
				// No dropped messages
				check.Equal(pCounts.Csnp().Dropped().State(), uint32(0)),
				check.Equal(pCounts.Psnp().Dropped().State(), uint32(0)),
				check.Equal(pCounts.Lsp().Dropped().State(), uint32(0)),
				check.Equal(pCounts.Iih().Dropped().State(), uint32(0)),
			} {
				t.Run(vd.RelPath(pCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})

		t.Run("circuit_counters", func(t *testing.T) {
			// Only adjChanges and adjNumber should have gone up - others should still be 0
			cCounts := port1ISIS.CircuitCounters()
			for _, vd := range []check.Validator{
				check.NotEqual(cCounts.AdjChanges().State(), uint32(0)),
				check.NotEqual(cCounts.AdjNumber().State(), uint32(0)),
				check.Equal(cCounts.AuthFails().State(), uint32(0)),
				check.Equal(cCounts.AuthTypeFails().State(), uint32(0)),
				check.Equal(cCounts.IdFieldLenMismatches().State(), uint32(0)),
				check.Equal(cCounts.LanDisChanges().State(), uint32(0)),
				check.Equal(cCounts.MaxAreaAddressMismatches().State(), uint32(0)),
				check.Equal(cCounts.RejectedAdj().State(), uint32(0)),
			} {
				t.Run(vd.RelPath(cCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})

		t.Run("level_counters", func(t *testing.T) {
			// Error counters should still be zero
			sysCounts := isisRoot.Level(2).SystemLevelCounters()
			for _, vd := range []check.Validator{
				check.Equal(sysCounts.AuthFails().State(), uint32(0)),
				check.Equal(sysCounts.AuthTypeFails().State(), uint32(0)),
				check.Equal(sysCounts.CorruptedLsps().State(), uint32(0)),
				check.Equal(sysCounts.DatabaseOverloads().State(), uint32(0)),
				check.Equal(sysCounts.ExceedMaxSeqNums().State(), uint32(0)),
				check.Equal(sysCounts.IdLenMismatch().State(), uint32(0)),
				check.Equal(sysCounts.LspErrors().State(), uint32(0)),
				check.Equal(sysCounts.MaxAreaAddressMismatches().State(), uint32(0)),
				check.Equal(sysCounts.OwnLspPurges().State(), uint32(0)),
				check.Equal(sysCounts.SeqNumSkips().State(), uint32(0)),
				check.Predicate(sysCounts.SpfRuns().State(), fmt.Sprintf("want > %v", spfBefore), func(got uint32) bool {
					return got > spfBefore
				}),
			} {
				t.Run(vd.RelPath(sysCounts), func(t *testing.T) {
					if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
						t.Error(err)
					}
				})
			}
		})
	})
}

// TestHelloPadding tests several different hello padding modes to confirm they all work.
func TestHelloPadding(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode oc.E_Isis_HelloPaddingType
		skip string
	}{
		{
			name: "disabled",
			mode: oc.Isis_HelloPaddingType_DISABLE,
		}, {
			name: "strict",
			mode: oc.Isis_HelloPaddingType_STRICT,
		}, {
			name: "adaptive",
			mode: oc.Isis_HelloPaddingType_ADAPTIVE,
		}, {
			name: "loose",
			mode: oc.Isis_HelloPaddingType_LOOSE,
			// TODO: Skip based on deviations.
			skip: "Unsupported",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			ts := session.MustNew(t).WithISIS()
			ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
				global := isis.GetOrCreateGlobal()
				global.HelloPadding = tc.mode
			}, func(isis *ixnet.ISIS) {
				isis.WithHelloPaddingEnabled(tc.mode != oc.Isis_HelloPaddingType_DISABLE)
			})
			ts.PushAndStart(t)
			_, err := ts.AwaitAdjacency()
			if err != nil {
				t.Fatalf("No IS-IS adjacency formed: %v", err)
			}
			telemPth := session.ISISPath().Global()
			var vd check.Validator
			if tc.mode == oc.Isis_HelloPaddingType_STRICT {
				vd = EqualToDefault(telemPth.HelloPadding().State(), oc.Isis_HelloPaddingType_STRICT)
			} else {
				vd = check.Equal(telemPth.HelloPadding().State(), tc.mode)
			}
			if err := vd.Check(ts.DUTClient); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestAuthentication verifies that with authentication enabled or disabled we can still establish
// an IS-IS session with the ATE.
func TestAuthentication(t *testing.T) {
	const password = "google"
	for _, tc := range []struct {
		name    string
		mode    oc.E_IsisTypes_AUTH_MODE
		enabled bool
	}{
		{name: "enabled:md5", mode: oc.IsisTypes_AUTH_MODE_MD5, enabled: true},
		{name: "enabled:text", mode: oc.IsisTypes_AUTH_MODE_TEXT, enabled: true},
		{name: "disabled", mode: oc.IsisTypes_AUTH_MODE_TEXT, enabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := session.MustNew(t).WithISIS()
			ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
				level := isis.GetOrCreateLevel(2)
				level.Enabled = ygot.Bool(true)
				auth := level.GetOrCreateAuthentication()
				auth.Enabled = ygot.Bool(true)
				auth.AuthMode = tc.mode
				auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
				auth.AuthPassword = ygot.String(password)
				for _, intf := range isis.Interface {
					intf.GetOrCreateLevel(2).GetOrCreateHelloAuthentication().Enabled = ygot.Bool(tc.enabled)
					if tc.enabled {
						intf.GetLevel(2).GetHelloAuthentication().AuthPassword = ygot.String("google")
						intf.GetLevel(2).GetHelloAuthentication().AuthMode = tc.mode
						intf.GetLevel(2).GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
					}
				}
			}, func(isis *ixnet.ISIS) {
				if tc.enabled {
					switch tc.mode {
					case oc.IsisTypes_AUTH_MODE_TEXT:
						isis.WithAuthPassword(password)
					case oc.IsisTypes_AUTH_MODE_MD5:
						isis.WithAuthMD5(password)
					default:
						t.Fatalf("test case has bad mode: %v", tc.mode)
					}
				} else {
					isis.WithAuthDisabled()
				}
			})
			ts.PushAndStart(t)
			ts.MustAdjacency(t)
		})
	}
}

// TestTraffic has the ATE advertise some routes and verifies that traffic sent to the DUT is routed
// appropriately.
func TestTraffic(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
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

	ts.ConfigISIS(func(isis *oc.NetworkInstance_Protocol_Isis) {
		// disable global hello padding on the DUT
		global := isis.GetOrCreateGlobal()
		global.HelloPadding = oc.Isis_HelloPaddingType_DISABLE
	}, func(isis *ixnet.ISIS) {
		// disable global hello padding on the ATE
		isis.WithHelloPaddingEnabled(false)
	})

	ate := ts.ATE
	// We generate traffic entering along port2 and destined for port1
	srcIntf := ts.MustATEInterface(t, "port2")
	dstIntf := ts.MustATEInterface(t, "port1")
	// net is a simulated network containing the addresses specified by targetNetwork
	net := dstIntf.AddNetwork("net")
	net.IPv4().WithAddress(targetNetwork.IPv4CIDR()).WithCount(1)
	net.IPv6().WithAddress(targetNetwork.IPv6CIDR()).WithCount(1)
	net.ISIS().WithIPReachabilityExternal().WithIPReachabilityMetric(10)
	t.Logf("Starting protocols on ATE...")
	ts.PushAndStart(t)
	defer ts.ATETop.StopProtocols(t)
	ts.MustAdjacency(t)
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
