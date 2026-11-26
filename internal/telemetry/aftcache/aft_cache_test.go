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

package aftcache

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/cache"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func pathElem(name string, kv ...string) *gnmipb.PathElem {
	e := &gnmipb.PathElem{Name: name}
	if len(kv) > 0 {
		e.Key = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			e.Key[kv[i]] = kv[i+1]
		}
	}
	return e
}

func prefixNotification(ni, table, entry, prefix string, nhgID uint64, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	return &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: ts,
				Atomic:    atomic,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Target: "dut",
					Elem: []*gnmipb.PathElem{
						pathElem("network-instances"),
						pathElem("network-instance", "name", ni),
						pathElem("afts"),
						pathElem(table),
						pathElem(entry, "prefix", prefix),
					},
				},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: prefix}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: prefix}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("next-hop-group")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("origin-protocol")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "BGP"}}},
				},
			},
		},
	}
}

func nhgNotification(ni string, nhgID uint64, nhWeights map[uint64]uint64, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	updates := []*gnmipb.Update{
		{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("id")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
		{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("id")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
	}
	for nhID, w := range nhWeights {
		idStr := fmt.Sprintf("%d", nhID)
		updates = append(updates,
			&gnmipb.Update{
				Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", idStr), pathElem("state"), pathElem("index")}},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhID}},
			},
			&gnmipb.Update{
				Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", idStr), pathElem("state"), pathElem("weight")}},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: w}},
			},
		)
	}
	return &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: ts,
				Atomic:    atomic,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Target: "dut",
					Elem: []*gnmipb.PathElem{
						pathElem("network-instances"),
						pathElem("network-instance", "name", ni),
						pathElem("afts"),
						pathElem("next-hop-groups"),
						pathElem("next-hop-group", "id", fmt.Sprintf("%d", nhgID)),
					},
				},
				Update: updates,
			},
		},
	}
}

func nhNotification(ni string, nhID uint64, ip, intf string, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	return &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: ts,
				Atomic:    atomic,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Target: "dut",
					Elem: []*gnmipb.PathElem{
						pathElem("network-instances"),
						pathElem("network-instance", "name", ni),
						pathElem("afts"),
						pathElem("next-hops"),
						pathElem("next-hop", "index", fmt.Sprintf("%d", nhID)),
					},
				},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhID}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("ip-address")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: ip}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("interface-ref"), pathElem("state"), pathElem("interface")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: intf}}},
				},
			},
		},
	}
}

func TestAFTCacheAtomicLifecycleAndStoppingConditions(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{
		Cache:             c,
		start:             time.Now(),
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
	}

	// Add NHs, NHGs, and V4/V6 prefixes.
	for i, n := range []*gnmipb.SubscribeResponse{
		nhNotification("DEFAULT", 1, "192.0.2.2", "Ethernet3/1", 100, true),
		nhNotification("DEFAULT", 2, "192.0.2.6", "Ethernet2/1", 100, true),
		nhNotification("DEFAULT", 3, "2001:db8::2", "Ethernet3/1", 100, true),
		nhNotification("DEFAULT", 4, "fe80::200:2ff:fe02:202", "Ethernet3/1", 100, true),
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 100, true),
		nhgNotification("DEFAULT", 20, map[uint64]uint64{3: 1}, 100, true),
		nhgNotification("DEFAULT", 30, map[uint64]uint64{4: 1}, 100, true),
		nhgNotification("DEFAULT", 99, nil, 100, true), // empty NHG like startup state
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
		prefixNotification("DEFAULT", "ipv6-unicast", "ipv6-entry", "2001:db8:1::1/128", 20, 100, true),
		prefixNotification("DEFAULT", "ipv6-unicast", "ipv6-entry", "2001:db8:2::1/128", 30, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	// Stale update must return cache.ErrStale and not overwrite.
	staleResp := prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 999, 50, true)
	if err := c.addAFTNotification(staleResp); !errors.Is(err, cache.ErrStale) {
		t.Fatalf("addAFTNotification(stale ts=50) = %v, want %v", err, cache.ErrStale)
	}

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if got := aft.Prefixes["198.51.100.1/32"]; got != 10 {
		t.Errorf("ToAFT() Prefixes[%q] = %d, want 10", "198.51.100.1/32", got)
	}
	wantNH := &aftNextHop{IP: "192.0.2.2", IntfName: "Ethernet3/1"}
	if diff := cmp.Diff(wantNH, aft.NextHops[1]); diff != "" {
		t.Errorf("ToAFT() NextHops[1] diff (-want +got):\n%s", diff)
	}

	// Verify memoized InitialSyncStoppingCondition passes for BGP prefixes.
	want := map[string]bool{"198.51.100.1/32": true, "2001:db8:1::1/128": true}
	sc := InitialSyncStoppingCondition(t, nil, want, map[string]bool{"192.0.2.2": true, "192.0.2.6": true}, map[string]bool{"2001:db8::2": true})
	done, err := sc.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("InitialSyncStoppingCondition() = (%v, %v), want (true, nil)", done, err)
	}

	// Verify AssertNextHopCount.
	nhCountHook := AssertNextHopCount(t, nil, map[string]bool{"198.51.100.1/32": true}, 2)
	done, err = nhCountHook.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("AssertNextHopCount(wantNHCount=2) = (%v, %v), want (true, nil)", done, err)
	}

	// Delete individual prefix and verify DeletionStoppingCondition.
	delNotif := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Elem: []*gnmipb.PathElem{
						pathElem("network-instances"),
						pathElem("network-instance", "name", "DEFAULT"),
						pathElem("afts"),
					},
				},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.1/32")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(delNotif); err != nil {
		t.Fatalf("addAFTNotification(delete 198.51.100.1/32) unexpected error: %v", err)
	}
	delHook := DeletionStoppingCondition(t, nil, map[string]bool{"198.51.100.1/32": true})
	done, err = delHook.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("DeletionStoppingCondition(198.51.100.1/32) = (%v, %v), want (true, nil)", done, err)
	}
}

func TestAddAFTNotificationNonAtomicRejected(t *testing.T) {
	tests := []struct {
		name  string
		notif *gnmipb.SubscribeResponse
	}{
		{
			name:  "ipv4_entry",
			notif: prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "203.0.113.1/32", 42, 100, false),
		},
		{
			name:  "ipv6_entry",
			notif: prefixNotification("DEFAULT", "ipv6-unicast", "ipv6-entry", "2001:db8:2::1/128", 42, 100, false),
		},
		{
			name:  "next_hop_group",
			notif: nhgNotification("DEFAULT", 42, map[uint64]uint64{1: 1}, 100, false),
		},
		{
			name:  "next_hop",
			notif: nhNotification("DEFAULT", 1, "192.0.2.2", "Ethernet1", 100, false),
		},
		{
			name: "short_prefix_non_atomic",
			notif: &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 100,
						Atomic:    false,
						Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
							pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"), pathElem("afts"),
						}},
						Update: []*gnmipb.Update{
							{
								Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "203.0.113.1/32"), pathElem("state"), pathElem("prefix")}},
								Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "203.0.113.1/32"}},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newAFTCache("dut", "DEFAULT")
			if err := c.addAFTNotification(tc.notif); !errors.Is(err, errNonAtomicUpdate) {
				t.Errorf("addAFTNotification(%s) = %v, want %v", tc.name, err, errNonAtomicUpdate)
			}
		})
	}

	// Non-atomic updates for a different network-instance are ignored.
	c := newAFTCache("dut", "DEFAULT")
	otherNI := prefixNotification("OTHER_VRF", "ipv4-unicast", "ipv4-entry", "203.0.113.1/32", 42, 100, false)
	if err := c.addAFTNotification(otherNI); err != nil {
		t.Errorf("addAFTNotification(OTHER_VRF non-atomic) = %v, want nil", err)
	}
}

func TestAddAFTNotificationShortPrefixAtomic(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}

	// Seed initial entries with 2 NHG members and IP+Intf on NH 1.
	for i, n := range []*gnmipb.SubscribeResponse{
		nhNotification("DEFAULT", 1, "192.0.2.2", "Ethernet1", 100, true),
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 100, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "203.0.113.1/32", 10, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	// Send one short-prefix atomic notification updating prefix, NHG 10 (only
	// NH 1), and NH 1 (only IP, dropping Ethernet1). Every touched entry must be
	// replaced as a whole.
	shortAtomic := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Atomic:    true,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"), pathElem("afts"),
				}},
				Update: []*gnmipb.Update{
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "203.0.113.1/32"), pathElem("state"), pathElem("prefix")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "203.0.113.1/32"}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "203.0.113.1/32"), pathElem("state"), pathElem("next-hop-group")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"), pathElem("state"), pathElem("id")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"), pathElem("next-hops"), pathElem("next-hop", "index", "1"), pathElem("state"), pathElem("index")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"), pathElem("next-hops"), pathElem("next-hop", "index", "1"), pathElem("state"), pathElem("weight")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "1"), pathElem("state"), pathElem("index")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "1"), pathElem("state"), pathElem("ip-address")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "192.0.2.9"}},
					},
				},
			},
		},
	}
	if err := c.addAFTNotification(shortAtomic); err != nil {
		t.Fatalf("addAFTNotification(shortAtomic) unexpected error: %v", err)
	}
	if c.shortPrefixAtomicCount != 1 {
		t.Errorf("shortPrefixAtomicCount = %d, want 1", c.shortPrefixAtomicCount)
	}
	c.logMetadata(t, ss.start, "[test]")

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if got := aft.Prefixes["203.0.113.1/32"]; got != 10 {
		t.Errorf("ToAFT() Prefixes[%q] = %d, want 10", "203.0.113.1/32", got)
	}
	wantNHG := &aftNextHopGroup{NHIDs: []uint64{1}, NHWeights: map[uint64]uint64{1: 1}}
	if diff := cmp.Diff(wantNHG, aft.NextHopGroups[10]); diff != "" {
		t.Errorf("ToAFT() NextHopGroups[10] diff (-want +got):\n%s", diff)
	}
	wantNH := &aftNextHop{IP: "192.0.2.9"}
	if diff := cmp.Diff(wantNH, aft.NextHops[1]); diff != "" {
		t.Errorf("ToAFT() NextHops[1] diff (-want +got):\n%s", diff)
	}
}

func TestAFTCacheAtomicReplaceAndDeleteSemantics(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}

	for i, n := range []*gnmipb.SubscribeResponse{
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.5/32", 10, 100, true),
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 100, true),
		nhNotification("DEFAULT", 7, "192.0.2.7", "Ethernet7/1", 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	// 1. Atomic update replaces the entire NHG entry without retaining stale
	// members.
	if err := c.addAFTNotification(nhgNotification("DEFAULT", 10, map[uint64]uint64{3: 1}, 150, true)); err != nil {
		t.Fatalf("addAFTNotification(atomic NHG 10 replace) unexpected error: %v", err)
	}
	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	wantNHG := &aftNextHopGroup{NHIDs: []uint64{3}, NHWeights: map[uint64]uint64{3: 1}}
	if diff := cmp.Diff(wantNHG, aft.NextHopGroups[10]); diff != "" {
		t.Errorf("ToAFT() NextHopGroups[10] diff (-want +got):\n%s", diff)
	}

	// 2. Stale sub-entry delete (ts 50 < 100) must not remove entries.
	staleDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 50,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
					pathElem("afts"), pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.5/32"),
				}},
				Delete: []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("origin-protocol")}}},
			},
		},
	}
	if err := c.addAFTNotification(staleDel); err != nil {
		t.Fatalf("addAFTNotification(staleDel ts=50) unexpected error: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if got := aft.Prefixes["198.51.100.5/32"]; got != 10 {
		t.Errorf("ToAFT() after stale delete Prefixes[%q] = %d, want 10", "198.51.100.5/32", got)
	}

	// 3. Per gNMI spec Section 2.1.1, any delete below an atomic entry key
	// removes the whole entry.
	subEntryDels := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"), pathElem("afts"),
				}},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.5/32"), pathElem("state"), pathElem("origin-protocol")}},
					{Elem: []*gnmipb.PathElem{pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"), pathElem("next-hops"), pathElem("next-hop", "index", "3"), pathElem("state"), pathElem("weight")}},
					{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "7"), pathElem("state"), pathElem("ip-address")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(subEntryDels); err != nil {
		t.Fatalf("addAFTNotification(subEntryDels) unexpected error: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if _, ok := aft.Prefixes["198.51.100.5/32"]; ok {
		t.Errorf("ToAFT() after sub-entry delete still contains prefix %q", "198.51.100.5/32")
	}
	if _, ok := aft.NextHopGroups[10]; ok {
		t.Errorf("ToAFT() after sub-entry delete still contains NHG 10")
	}
	if _, ok := aft.NextHops[7]; ok {
		t.Errorf("ToAFT() after sub-entry delete still contains NH 7")
	}

	// 4. Atomic delete is rejected.
	atomicDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 400,
				Atomic:    true,
				Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pathElem("afts")}}},
			},
		},
	}
	if err := c.addAFTNotification(atomicDel); err == nil {
		t.Errorf("addAFTNotification(atomic delete) = nil, want non-nil error")
	}
}

func TestAFTDataFilterByPrefixes(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	for i, n := range []*gnmipb.SubscribeResponse{
		nhNotification("DEFAULT", 1, "192.0.2.2", "Ethernet1", 100, true),
		nhNotification("DEFAULT", 2, "192.0.2.6", "Ethernet2", 100, true),
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true),
		nhgNotification("DEFAULT", 20, map[uint64]uint64{2: 1}, 100, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.2/32", 20, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	got := aft.FilterByPrefixes(map[string]bool{
		"198.51.100.1/32":  true,
		"198.51.100.99/32": true, // absent prefix should be ignored
	})
	want := &AFTData{
		Prefixes: map[string]uint64{"198.51.100.1/32": 10},
		NextHopGroups: map[uint64]*aftNextHopGroup{
			10: {NHIDs: []uint64{1}, NHWeights: map[uint64]uint64{1: 1}},
		},
		NextHops: map[uint64]*aftNextHop{
			1: {IP: "192.0.2.2", IntfName: "Ethernet1"},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FilterByPrefixes() diff (-want +got):\n%s", diff)
	}
}

func TestVerifyAtomicFlagHook(t *testing.T) {
	hook := VerifyAtomicFlagHook(t)
	c := newAFTCache("dut", "DEFAULT")

	tests := []struct {
		name    string
		resp    *gnmipb.SubscribeResponse
		wantErr bool
	}{
		{
			name: "sync_response_ignored",
			resp: &gnmipb.SubscribeResponse{Response: &gnmipb.SubscribeResponse_SyncResponse{SyncResponse: true}},
		},
		{
			name: "atomic_update_passes",
			resp: prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
		},
		{
			name:    "non_atomic_update_fails",
			resp:    prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, false),
			wantErr: true,
		},
		{
			name: "non_atomic_delete_passes",
			resp: &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 200,
						Atomic:    false,
						Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pathElem("ipv4-entry", "prefix", "198.51.100.1/32")}}},
					},
				},
			},
		},
		{
			name: "atomic_delete_fails",
			resp: &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 200,
						Atomic:    true,
						Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pathElem("ipv4-entry", "prefix", "198.51.100.1/32")}}},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := hook.NotificationFunc(c, tc.resp)
			if (err != nil) != tc.wantErr {
				t.Errorf("VerifyAtomicFlagHook(%s) = %v, wantErr=%v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestWithDebugAndWaitForNotification(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := (&AFTStreamSession{
		Cache:             c,
		start:             time.Now(),
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
	}).WithDebug()

	if !ss.debugMode {
		t.Fatalf("WithDebug() debugMode = false, want true")
	}

	syncResp := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_SyncResponse{SyncResponse: true},
	}
	addResp := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 100,
				Update: []*gnmipb.Update{
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{
							pathElem("ipv4-unicast"),
							pathElem("ipv4-entry", "prefix", "198.51.100.10/32"),
							pathElem("state"),
							pathElem("next-hop-group"),
						}},
						Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}},
					},
				},
			},
		},
	}
	delResp := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{
						pathElem("ipv4-unicast"),
						pathElem("ipv4-entry", "prefix", "198.51.100.20/32"),
					}},
				},
			},
		},
	}
	ss.notifications = []*gnmipb.SubscribeResponse{syncResp, addResp, delResp}

	if got := len(ss.Notifications()); got != 3 {
		t.Errorf("len(Notifications()) = %d, want 3", got)
	}

	// Matches both update and delete while skipping SyncResponse.
	waitBoth := WaitForNotification(t, NotificationExpectation{
		AddPrefix:        "198.51.100.10/32",
		DeletePrefix:     "198.51.100.20/32",
		NotificationWait: time.Second,
	})
	done, err := waitBoth.PeriodicFunc(ss)
	if err != nil || !done {
		t.Errorf("WaitForNotification(add+delete) = (%v, %v), want (true, nil)", done, err)
	}

	// Times out cleanly when expected prefix is not present.
	waitMissing := WaitForNotification(t, NotificationExpectation{
		AddPrefix:        "203.0.113.99/32",
		NotificationWait: time.Millisecond,
	})
	time.Sleep(5 * time.Millisecond)
	done, err = waitMissing.PeriodicFunc(ss)
	if err == nil || done {
		t.Errorf("WaitForNotification(missing) = (%v, %v), want (false, non-nil timeout error)", done, err)
	}
}

func TestAFTCacheScale(t *testing.T) {
	const routeCount = 200000
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{
		Cache:             c,
		start:             time.Now(),
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
	}
	if err := c.addAFTNotification(nhNotification("DEFAULT", 1, "192.0.2.2", "Ethernet3/1", 100, true)); err != nil {
		t.Fatalf("addAFTNotification(NH 1) unexpected error: %v", err)
	}
	if err := c.addAFTNotification(nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true)); err != nil {
		t.Fatalf("addAFTNotification(NHG 10) unexpected error: %v", err)
	}

	want := make(map[string]bool, routeCount)
	resp := prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "", 10, 100, true)
	prefixKey := resp.GetUpdate().GetPrefix().GetElem()[4].GetKey()
	prefixVal := resp.GetUpdate().GetUpdate()[0].GetVal().Value.(*gnmipb.TypedValue_StringVal)
	statePrefixVal := resp.GetUpdate().GetUpdate()[1].GetVal().Value.(*gnmipb.TypedValue_StringVal)

	for i := 0; i < routeCount; i++ {
		// RFC 6598 (100.64.0.0/10) provides 4M documentation-approved test
		// addresses.
		prefix := fmt.Sprintf("100.%d.%d.%d/32", 64+((i>>16)&0x3f), (i>>8)&0xff, i&0xff)
		want[prefix] = true
		prefixKey["prefix"] = prefix
		prefixVal.StringVal = prefix
		statePrefixVal.StringVal = prefix
		if err := c.addAFTNotification(resp); err != nil {
			t.Fatalf("addAFTNotification(route %d %q) unexpected error: %v", i, prefix, err)
		}
	}

	sc := InitialSyncStoppingCondition(t, nil, want, map[string]bool{"192.0.2.2": true}, nil)
	done, err := sc.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("InitialSyncStoppingCondition(%d routes) = (%v, %v), want (true, nil)", routeCount, done, err)
	}
}

func BenchmarkAFTCacheIngest(b *testing.B) {
	c := newAFTCache("dut", "DEFAULT")
	prefixes := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		prefixes[i] = fmt.Sprintf("100.%d.%d.%d/32", 64+((i>>16)&0x3f), (i>>8)&0xff, i&0xff)
	}
	resp := prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "", 10, 100, true)
	prefixKey := resp.GetUpdate().GetPrefix().GetElem()[4].GetKey()
	prefixVal := resp.GetUpdate().GetUpdate()[0].GetVal().Value.(*gnmipb.TypedValue_StringVal)
	statePrefixVal := resp.GetUpdate().GetUpdate()[1].GetVal().Value.(*gnmipb.TypedValue_StringVal)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prefix := prefixes[i]
		prefixKey["prefix"] = prefix
		prefixVal.StringVal = prefix
		statePrefixVal.StringVal = prefix
		if err := c.addAFTNotification(resp); err != nil {
			b.Fatalf("addAFTNotification(%d %q) unexpected error: %v", i, prefix, err)
		}
	}
}

func TestAddAFTNotificationStatePrefixMismatch(t *testing.T) {
	tests := []struct {
		name   string
		badVal string
	}{
		{name: "empty_state_prefix", badVal: ""},
		{name: "mismatched_state_prefix", badVal: "198.51.100.99/32"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newAFTCache("dut", "DEFAULT")
			n := &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 100,
						Atomic:    true,
						Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
							pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
							pathElem("afts"), pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.1/32"),
						}},
						Update: []*gnmipb.Update{
							{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: tc.badVal}}},
							{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("next-hop-group")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}}},
						},
					},
				},
			}
			if err := c.addAFTNotification(n); err == nil {
				t.Errorf("addAFTNotification(state/prefix=%q) = nil, want non-nil error", tc.badVal)
			}
		})
	}
}

func TestToAFTMissingRequiredPrefixFields(t *testing.T) {
	tests := []struct {
		name    string
		updates []*gnmipb.Update
	}{
		{
			name: "missing_nhg",
			updates: []*gnmipb.Update{
				{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "198.51.100.1/32"}}},
			},
		},
		{
			name: "missing_state_prefix",
			updates: []*gnmipb.Update{
				{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("next-hop-group")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newAFTCache("dut", "DEFAULT")
			ss := &AFTStreamSession{Cache: c, start: time.Now()}
			n := &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 100,
						Atomic:    true,
						Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
							pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
							pathElem("afts"), pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.1/32"),
						}},
						Update: tc.updates,
					},
				},
			}
			if err := c.addAFTNotification(n); err != nil {
				t.Fatalf("addAFTNotification(%s) unexpected error: %v", tc.name, err)
			}
			if _, err := ss.ToAFT(t, nil); err == nil {
				t.Errorf("ToAFT(%s) error = nil, want non-nil error", tc.name)
			}
		})
	}
}

func TestToAFTOmitsEmptyNextHop(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	emptyNH := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 100,
				Atomic:    true,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
					pathElem("afts"), pathElem("next-hops"), pathElem("next-hop", "index", "1"),
				}},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}}},
				},
			},
		},
	}
	for i, n := range []*gnmipb.SubscribeResponse{
		emptyNH,
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}
	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if got, ok := aft.NextHops[1]; ok {
		t.Errorf("ToAFT() NextHops[1] = %+v, want omitted", got)
	}
	hook := AssertNextHopCount(t, nil, map[string]bool{"198.51.100.1/32": true}, 1)
	if done, err := hook.PeriodicFunc(ss); err != nil || done {
		t.Errorf("AssertNextHopCount(empty NH) = (%v, %v), want (false, nil)", done, err)
	}
}

func TestToAFTNHGWeightCountMismatch(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	missingWeightNHG := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 100,
				Atomic:    true,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
					pathElem("afts"), pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"),
				}},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("state"), pathElem("id")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 10}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "1"), pathElem("state"), pathElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "2"), pathElem("state"), pathElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 2}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("next-hops"), pathElem("next-hop", "index", "2"), pathElem("state"), pathElem("weight")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}}},
				},
			},
		},
	}
	if err := c.addAFTNotification(missingWeightNHG); err != nil {
		t.Fatalf("addAFTNotification(missingWeightNHG) unexpected error: %v", err)
	}
	if _, err := ss.ToAFT(t, nil); err == nil {
		t.Errorf("ToAFT(NHG 10 missing weight) error = nil, want non-nil error")
	}
}

func TestAddAFTNotificationTimestampBoundaries(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	for i, n := range []*gnmipb.SubscribeResponse{
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 300, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.2/32", 10, 200, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	// Older atomic update (ts=250 < 300) is rejected as ErrStale without
	// modifying the cached entry.
	staleResp := prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 99, 250, true)
	if err := c.addAFTNotification(staleResp); !errors.Is(err, cache.ErrStale) {
		t.Errorf("addAFTNotification(ts=250 < 300) = %v, want %v", err, cache.ErrStale)
	}

	// Same-timestamp delete (ts=200 == 200) deletes .2 cleanly.
	sameTSDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"), pathElem("afts"),
				}},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.2/32")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(sameTSDel); err != nil {
		t.Fatalf("addAFTNotification(sameTSDel ts=200) unexpected error: %v", err)
	}

	// Zero-timestamp delete must not override a positive-timestamp entry
	// (.1 has ts=300).
	zeroTSDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 0,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"), pathElem("afts"),
				}},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pathElem("ipv4-unicast"), pathElem("ipv4-entry", "prefix", "198.51.100.1/32")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(zeroTSDel); err != nil {
		t.Fatalf("addAFTNotification(zeroTSDel ts=0) unexpected error: %v", err)
	}

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	wantPrefixes := map[string]uint64{"198.51.100.1/32": 10}
	if diff := cmp.Diff(wantPrefixes, aft.Prefixes); diff != "" {
		t.Errorf("ToAFT() Prefixes diff (-want +got):\n%s", diff)
	}
}

func TestAddAFTNotificationConditionalNHGAndAncestorDelete(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	for i, n := range []*gnmipb.SubscribeResponse{
		nhgNotification("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true),
		prefixNotification("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification(seed[%d]) unexpected error: %v", i, err)
		}
	}

	// Conditional NHG replacing regular NHG 10 must evict the old entry.
	cnhgNotif := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Atomic:    true,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT"),
					pathElem("afts"), pathElem("next-hop-groups"), pathElem("next-hop-group", "id", "10"),
				}},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pathElem("conditional"), pathElem("condition", "id", "1"), pathElem("state"), pathElem("next-hop-group")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 20}}},
				},
			},
		},
	}
	if err := c.addAFTNotification(cnhgNotif); err != nil {
		t.Fatalf("addAFTNotification(conditional NHG 10) unexpected error: %v", err)
	}
	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if got, ok := aft.NextHopGroups[10]; ok {
		t.Errorf("ToAFT() NextHopGroups[10] = %+v, want evicted", got)
	}

	// Ancestor delete at network-instance[name=DEFAULT] clears remaining
	// cached entries.
	ancestorDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 300,
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pathElem("network-instances"), pathElem("network-instance", "name", "DEFAULT")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(ancestorDel); err != nil {
		t.Fatalf("addAFTNotification(ancestorDel) unexpected error: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT() unexpected error: %v", err)
	}
	if len(aft.Prefixes) != 0 {
		t.Errorf("ToAFT() after ancestor delete Prefixes = %+v, want empty", aft.Prefixes)
	}
}
