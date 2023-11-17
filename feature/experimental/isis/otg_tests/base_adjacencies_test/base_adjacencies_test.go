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
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/check"
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

// EqualToDefault is the same as check.Equal unless the AllowNilForDefaults
// deviation is set, in which case it uses check.EqualOrNil to allow the device
// to return a nil value. This should only be used when `val` is the default
// for this particular query.
func EqualToDefault[T any](query ygnmi.SingletonQuery[T], val T, missingValueForDefaults bool) check.Validator {
	if missingValueForDefaults {
		return check.EqualOrNil(query, val)
	}
	return check.Equal(query, val)
}

// CheckPresence check for the leaf presense only when missingValueForDefaults is false.
func CheckPresence(query ygnmi.SingletonQuery[uint32], missingValueForDefaults bool) check.Validator {
	if !missingValueForDefaults {
		return check.Present[uint32](query)
	}
	return check.Validate(query, func(vgot *ygnmi.Value[uint32]) error {
		return nil
	})
}

// TestBasic configures IS-IS on the DUT and confirms that the various values and defaults propagate
// then configures the ATE as well, waits for the adjacency to form, and checks that numerous
// counters and other values now have sensible values.
func TestBasic(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
	// Only push DUT config - no adjacency established yet
	if err := ts.PushDUT(context.Background(), t); err != nil {
		t.Fatalf("Unable to push initial DUT config: %v", err)
	}
	isisRoot := session.ISISPath(ts.DUT)
	port1ISIS := isisRoot.Interface(ts.DUTPort1.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		port1ISIS = isisRoot.Interface(ts.DUTPort1.Name() + ".0")
	}
	// There might be lag between when the instance name is set and when the
	// other parameters are set; we expect the total lag to be under one minute
	// There are about 14 RPCs executed in quick succession in this block.
	// Increasing the wait-time to 1 minute value to accommodate this.

	deadline := time.Now().Add(time.Minute)

	t.Run("read_config", func(t *testing.T) {
		checks := []check.Validator{
			check.Equal(isisRoot.Global().Net().State(), []string{"49.0001.1920.0000.2001.00"}),
			check.Equal(isisRoot.Global().LevelCapability().State(), oc.Isis_LevelType_LEVEL_2),
			check.Equal(port1ISIS.Enabled().State(), true),
			check.Equal(port1ISIS.CircuitType().State(), oc.Isis_CircuitType_POINT_TO_POINT),
		}

		// if MissingIsisInterfaceAfiSafiEnable is set, ignore enable flag check for AFI, SAFI at global level
		// and validate enable at interface level
		if deviations.MissingIsisInterfaceAfiSafiEnable(ts.DUT) {
			checks = append(checks,
				check.Equal(port1ISIS.Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true),
				check.Equal(port1ISIS.Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true))
		} else {
			checks = append(checks,
				check.Equal(isisRoot.Global().Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true),
				check.Equal(isisRoot.Global().Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled().State(), true))
		}

		// if ISISInterfaceLevel1DisableRequired is set, validate Level1 enabled false at interface level else validate Level2 enabled at global level
		if deviations.ISISInterfaceLevel1DisableRequired(ts.DUT) {
			checks = append(checks, check.Equal(port1ISIS.Level(1).Enabled().State(), false))
		} else {
			checks = append(checks, check.Equal(isisRoot.Level(2).Enabled().State(), true))
		}

		for _, vd := range checks {
			t.Run(vd.RelPath(isisRoot), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Error(err)
				}
			})
		}
	})

	missingValueForDefaults := deviations.MissingValueForDefaults(ts.DUT)
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
				EqualToDefault(pCounts.Csnp().Dropped().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Csnp().Processed().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Csnp().Received().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Csnp().Sent().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Psnp().Dropped().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Psnp().Processed().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Psnp().Received().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Psnp().Sent().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Lsp().Dropped().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Lsp().Processed().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Lsp().Received().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Lsp().Sent().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Iih().Dropped().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Iih().Processed().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(pCounts.Iih().Received().State(), uint32(0), missingValueForDefaults),
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
				EqualToDefault(cCounts.AdjChanges().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.AdjNumber().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.AuthFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.AuthTypeFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.IdFieldLenMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.LanDisChanges().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.MaxAreaAddressMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.RejectedAdj().State(), uint32(0), missingValueForDefaults),
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
				EqualToDefault(sysCounts.AuthFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.AuthTypeFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.CorruptedLsps().State(), uint32(0), missingValueForDefaults),
				CheckPresence(sysCounts.DatabaseOverloads().State(), missingValueForDefaults),
				EqualToDefault(sysCounts.ExceedMaxSeqNums().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.IdLenMismatch().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.LspErrors().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.MaxAreaAddressMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.OwnLspPurges().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.SeqNumSkips().State(), uint32(0), missingValueForDefaults),
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

	// Allow 1 Minute of lag between adjacency appearing and all data being populated
	t.Run("adjacency_state", func(t *testing.T) {
		// There are about 16 RPCs executed in quick succession in this block.
		// Increasing the wait-time value to accommodate this.
		deadline = time.Now().Add(time.Minute)
		adj := port1ISIS.Level(2).Adjacency(systemID)
		for _, vd := range []check.Validator{
			check.Equal(adj.AdjacencyState().State(), oc.Isis_IsisInterfaceAdjState_UP),
			check.Equal(adj.SystemId().State(), systemID),
			check.UnorderedEqual(adj.AreaAddress().State(), []string{session.ATEAreaAddress, session.DUTAreaAddress}, func(a, b string) bool { return a < b }),
			check.EqualOrNil(adj.DisSystemId().State(), "0000.0000.0000"),
			check.NotEqual(adj.LocalExtendedCircuitId().State(), uint32(0)),
			check.Equal(adj.MultiTopology().State(), false),
			check.Equal(adj.NeighborCircuitType().State(), oc.Isis_LevelType_LEVEL_2),
			check.NotEqual(adj.NeighborExtendedCircuitId().State(), uint32(0)),
			check.Equal(adj.NeighborIpv4Address().State(), session.ATEISISAttrs.IPv4),
			check.Predicate(adj.NeighborSnpa().State(), "Need a valid MAC address", func(got string) bool {
				mac, err := net.ParseMAC(got)
				return mac != nil && err == nil
			}),
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
				if strings.Contains(vd.Path(), "multi-topology") {
					if deviations.ISISMultiTopologyUnsupported(ts.DUT) {
						t.Skip("Multi-Topology Unsupported")
					}
				}
				if strings.Contains(vd.Path(), "restart-suppress") {
					if deviations.ISISRestartSuppressUnsupported(ts.DUT) {
						t.Skip("Restart-Suppress Unsupported")
					}
				}
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
		// There are about 3 RPCs executed in quick succession in this block.
		// Increasing the wait-time value to accommodate this.

		deadline = time.Now().Add(time.Second * 15)
		for _, vd := range []check.Validator{
			check.NotEqual(pCounts.Csnp().Processed().State(), uint32(0)),
			check.NotEqual(pCounts.Lsp().Processed().State(), uint32(0)),
		} {
			t.Run(vd.RelPath(pCounts), func(t *testing.T) {
				if err := vd.AwaitUntil(deadline, ts.DUTClient); err != nil {
					t.Fatalf("No messages in active adjacency after 5s: %v", err)
				}
			})
		}
		// There are about 16 RPCs executed in quick succession in this block.
		// Increasing the wait-time value to accommodate this.

		deadline = time.Now().Add(time.Minute)
		t.Run("packet_counters", func(t *testing.T) {
			pCounts := port1ISIS.Level(2).PacketCounters()
			for _, vd := range []check.Validator{
				check.NotEqual(pCounts.Csnp().Processed().State(), uint32(0)),
				check.NotEqual(pCounts.Csnp().Received().State(), uint32(0)),
				check.NotEqual(pCounts.Csnp().Sent().State(), uint32(0)),
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
				EqualToDefault(cCounts.AuthFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.AuthTypeFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.IdFieldLenMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.LanDisChanges().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.MaxAreaAddressMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(cCounts.RejectedAdj().State(), uint32(0), missingValueForDefaults),
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
				EqualToDefault(sysCounts.AuthFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.AuthTypeFails().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.CorruptedLsps().State(), uint32(0), missingValueForDefaults),
				CheckPresence(sysCounts.DatabaseOverloads().State(), missingValueForDefaults),
				EqualToDefault(sysCounts.ExceedMaxSeqNums().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.IdLenMismatch().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.LspErrors().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.MaxAreaAddressMismatches().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.OwnLspPurges().State(), uint32(0), missingValueForDefaults),
				EqualToDefault(sysCounts.SeqNumSkips().State(), uint32(0), missingValueForDefaults),
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
			})
			ts.ATEIntf1.Isis().Advanced().SetEnableHelloPadding(tc.mode != oc.Isis_HelloPaddingType_DISABLE)
			ts.PushAndStart(t)
			_, err := ts.AwaitAdjacency()
			if err != nil {
				t.Fatalf("No IS-IS adjacency formed: %v", err)
			}
			telemPth := session.ISISPath(ts.DUT).Global()
			var vd check.Validator
			missingValueForDefaults := deviations.MissingValueForDefaults(ts.DUT)
			if tc.mode == oc.Isis_HelloPaddingType_STRICT {
				vd = EqualToDefault(telemPth.HelloPadding().State(), oc.Isis_HelloPaddingType_STRICT, missingValueForDefaults)
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
			})
			if tc.enabled {
				switch tc.mode {
				case oc.IsisTypes_AUTH_MODE_TEXT:
					ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetAuthType("password")
					ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetPassword(password)
				case oc.IsisTypes_AUTH_MODE_MD5:
					ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetAuthType("md5")
					ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetMd5(password)
				default:
					t.Fatalf("test case has bad mode: %v", tc.mode)
				}
			}
			ts.PushAndStart(t)
			ts.MustAdjacency(t)
		})
	}
}

// TestTraffic has the ATE advertise some routes and verifies that traffic sent to the DUT is routed
// appropriately.
func TestTraffic(t *testing.T) {
	ts := session.MustNew(t).WithISIS()
	otg := ts.ATE.OTG()
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
		// configuring single topology for ISIS global ipv4 AF
		if deviations.ISISSingleTopologyRequired(ts.DUT) {
			afv6 := global.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
			afv6.GetOrCreateMultiTopology().SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
			afv6.GetOrCreateMultiTopology().SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
		}
	})
	ts.ATEIntf1.Isis().Advanced().SetEnableHelloPadding(false)

	// We generate traffic entering along port2 and destined for port1
	srcIntf := ts.MustATEInterface(t, "port2")
	dstIntf := ts.MustATEInterface(t, "port1")

	srcIpv4 := srcIntf.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := dstIntf.Isis().V4Routes().Add().SetName("netv4").SetLinkMetric(10).SetOriginType(gosnappi.IsisV4RouteRangeOriginType.EXTERNAL)
	netv4.Addresses().Add().SetAddress(targetNetwork.IPv4).SetPrefix(uint32(targetNetwork.IPv4Len))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := dstIntf.Isis().V6Routes().Add().SetName("netv6").SetLinkMetric(10).SetOriginType(gosnappi.IsisV6RouteRangeOriginType.EXTERNAL)
	netv6.Addresses().Add().SetAddress(targetNetwork.IPv6).SetPrefix(uint32(targetNetwork.IPv6Len))

	t.Log("Configuring traffic from ATE through DUT...")

	v4Flow := ts.ATETop.Flows().Add()
	v4Flow.SetName("v4Flow")
	v4Flow.TxRx().Device().SetTxNames([]string{srcIpv4.Name()}).SetRxNames([]string{netv4.Name()})

	v4FlowEth := v4Flow.Packet().Add().Ethernet()
	v4FlowEth.Src().SetValue(session.ATETrafficAttrs.MAC)

	v4FlowIp := v4Flow.Packet().Add().Ipv4()
	v4FlowIp.Src().SetValue(session.ATETrafficAttrs.IPv4)
	v4FlowIp.Dst().SetValue(targetNetwork.IPv4)

	v4Flow.Metrics().SetEnable(true)

	srcIpv6 := srcIntf.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	v6Flow := ts.ATETop.Flows().Add()
	v6Flow.SetName("v6Flow")
	v6Flow.TxRx().Device().SetTxNames([]string{srcIpv6.Name()}).SetRxNames([]string{netv6.Name()})

	v6FlowEth := v6Flow.Packet().Add().Ethernet()
	v6FlowEth.Src().SetValue(session.ATETrafficAttrs.MAC)

	v6FlowIp := v6Flow.Packet().Add().Ipv6()
	v6FlowIp.Src().SetValue(session.ATETrafficAttrs.IPv6)
	v6FlowIp.Dst().SetValue(targetNetwork.IPv6)

	// v6Flow.Duration().FixedPackets().SetPackets(100)
	v6Flow.Rate().SetPps(50)
	v6Flow.Size().SetFixed(128)
	v6Flow.Metrics().SetEnable(true)

	deadFlow := ts.ATETop.Flows().Add()
	deadFlow.SetName("deadFlow")
	deadFlow.TxRx().Device().SetTxNames([]string{srcIpv4.Name()}).SetRxNames([]string{netv4.Name()})

	deadFlowEth := deadFlow.Packet().Add().Ethernet()
	deadFlowEth.Src().SetValue(session.ATETrafficAttrs.MAC)

	deadFlowIp := deadFlow.Packet().Add().Ipv4()
	deadFlowIp.Src().SetValue(session.ATETrafficAttrs.IPv4)
	deadFlowIp.Dst().SetValue(deadNetwork.IPv4)

	// deadFlow.Duration().FixedPackets().SetPackets(100)
	deadFlow.Rate().SetPps(50)
	deadFlow.Size().SetFixed(128)
	deadFlow.Metrics().SetEnable(true)

	t.Log("Starting protocols on ATE...")
	ts.PushAndStart(t)
	ts.MustAdjacency(t)

	gnmi.Watch(t, otg, gnmi.OTG().IsisRouter("devIsis").Counters().Level2().InLsp().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
		time.Sleep(5 * time.Second)
		val, present := v.Val()
		return present && val >= 1
	}).Await(t)

	time.Sleep(10 * time.Second)
	// TODO: To match the exact IS-IS route prefix once this becomes available in otg

	// otg.Telemetry().IsisRouter("devIsis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(targetNetwork.IPv4).Watch(
	// 	t, 30*time.Second, func(val *otgtelemetry.QualifiedIsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix) bool {
	// 		return val.IsPresent()
	// 	}).Await(t)

	t.Log("Running traffic for 30s...")
	otg.StartTraffic(t)
	time.Sleep(time.Second * 30)
	otg.StopTraffic(t)

	t.Log("Checking telemetry...")
	otgutils.LogFlowMetrics(t, otg, ts.ATETop)

	v4Loss := ts.GetPacketLoss(t, v4Flow)
	v6Loss := ts.GetPacketLoss(t, v6Flow)
	deadLoss := ts.GetPacketLoss(t, deadFlow)
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
