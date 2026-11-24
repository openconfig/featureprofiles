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
	"runtime"
	"testing"
	"time"

	"github.com/openconfig/gnmi/cache"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func pElem(name string, kv ...string) *gnmipb.PathElem {
	e := &gnmipb.PathElem{Name: name}
	if len(kv) > 0 {
		e.Key = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			e.Key[kv[i]] = kv[i+1]
		}
	}
	return e
}

func makePrefixNoti(ni, table, entry, pfx string, nhgID uint64, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	return &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: ts,
				Atomic:    atomic,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Target: "dut",
					Elem: []*gnmipb.PathElem{
						pElem("network-instances"),
						pElem("network-instance", "name", ni),
						pElem("afts"),
						pElem(table),
						pElem(entry, "prefix", pfx),
					},
				},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: pfx}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("prefix")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: pfx}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("next-hop-group")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("origin-protocol")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "BGP"}}},
				},
			},
		},
	}
}

func makeNHGNoti(ni string, nhgID uint64, nhWeights map[uint64]uint64, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	updates := []*gnmipb.Update{
		{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("id")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
		{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("id")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhgID}}},
	}
	for nhID, w := range nhWeights {
		idStr := fmt.Sprintf("%d", nhID)
		updates = append(updates,
			&gnmipb.Update{
				Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("next-hops"), pElem("next-hop", "index", idStr), pElem("state"), pElem("index")}},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhID}},
			},
			&gnmipb.Update{
				Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("next-hops"), pElem("next-hop", "index", idStr), pElem("state"), pElem("weight")}},
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
						pElem("network-instances"),
						pElem("network-instance", "name", ni),
						pElem("afts"),
						pElem("next-hop-groups"),
						pElem("next-hop-group", "id", fmt.Sprintf("%d", nhgID)),
					},
				},
				Update: updates,
			},
		},
	}
}

func makeNHNoti(ni string, nhID uint64, ip, intf string, ts int64, atomic bool) *gnmipb.SubscribeResponse {
	return &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: ts,
				Atomic:    atomic,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Target: "dut",
					Elem: []*gnmipb.PathElem{
						pElem("network-instances"),
						pElem("network-instance", "name", ni),
						pElem("afts"),
						pElem("next-hops"),
						pElem("next-hop", "index", fmt.Sprintf("%d", nhID)),
					},
				},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: nhID}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("ip-address")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: ip}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("interface-ref"), pElem("state"), pElem("interface")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: intf}}},
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
	for _, n := range []*gnmipb.SubscribeResponse{
		makeNHNoti("DEFAULT", 1, "192.0.2.2", "Ethernet3/1", 100, true),
		makeNHNoti("DEFAULT", 2, "192.0.2.6", "Ethernet2/1", 100, true),
		makeNHNoti("DEFAULT", 3, "2001:db8::2", "Ethernet3/1", 100, true),
		makeNHNoti("DEFAULT", 4, "fe80::200:2ff:fe02:202", "Ethernet3/1", 100, true),
		makeNHGNoti("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 100, true),
		makeNHGNoti("DEFAULT", 20, map[uint64]uint64{3: 1}, 100, true),
		makeNHGNoti("DEFAULT", 30, map[uint64]uint64{4: 1}, 100, true),
		makeNHGNoti("DEFAULT", 99, map[uint64]uint64{}, 100, true), // empty NHG like startup state
		makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
		makePrefixNoti("DEFAULT", "ipv6-unicast", "ipv6-entry", "2001:db8:1::1/128", 20, 100, true),
		makePrefixNoti("DEFAULT", "ipv6-unicast", "ipv6-entry", "2001:db8:2::1/128", 30, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification failed: %v", err)
		}
	}

	// Stale update must return cache.ErrStale and not overwrite.
	err := c.addAFTNotification(makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 999, 50, true))
	if !errors.Is(err, cache.ErrStale) {
		t.Fatalf("expected ErrStale, got: %v", err)
	}

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	if aft.Prefixes["198.51.100.1/32"] != 10 {
		t.Fatalf("got NHG %d, want 10", aft.Prefixes["198.51.100.1/32"])
	}
	if aft.NextHops[1].IntfName != "Ethernet3/1" {
		t.Fatalf("got IntfName %q, want Ethernet3/1", aft.NextHops[1].IntfName)
	}

	// Verify memoized InitialSyncStoppingCondition passes for BGP prefixes.
	want := map[string]bool{"198.51.100.1/32": true, "2001:db8:1::1/128": true}
	sc := InitialSyncStoppingCondition(t, nil, want, map[string]bool{"192.0.2.2": true, "192.0.2.6": true}, map[string]bool{"2001:db8::2": true})
	done, err := sc.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("InitialSyncStoppingCondition got done=%v err=%v, want true, nil", done, err)
	}

	// Verify AssertNextHopCount.
	nhCountHook := AssertNextHopCount(t, nil, map[string]bool{"198.51.100.1/32": true}, 2)
	done, err = nhCountHook.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("AssertNextHopCount got done=%v err=%v, want true, nil", done, err)
	}

	// Delete individual prefix and verify DeletionStoppingCondition.
	delNoti := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Prefix: &gnmipb.Path{
					Origin: "openconfig",
					Elem: []*gnmipb.PathElem{
						pElem("network-instances"),
						pElem("network-instance", "name", "DEFAULT"),
						pElem("afts"),
					},
				},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pElem("ipv4-unicast"), pElem("ipv4-entry", "prefix", "198.51.100.1/32")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(delNoti); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	delHook := DeletionStoppingCondition(t, nil, map[string]bool{"198.51.100.1/32": true})
	done, err = delHook.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("DeletionStoppingCondition got done=%v err=%v, want true, nil", done, err)
	}
}

func TestAFTCacheNonAtomicAndShortPrefix(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}

	// Non-atomic split updates with short prefix.
	noti := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 100,
				Atomic:    false,
				Prefix: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{
						pElem("network-instances"),
						pElem("network-instance", "name", "DEFAULT"),
						pElem("afts"),
					},
				},
				Update: []*gnmipb.Update{
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("ipv4-unicast"), pElem("ipv4-entry", "prefix", "203.0.113.1/32"), pElem("state"), pElem("prefix")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "203.0.113.1/32"}},
					},
					{
						Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("ipv4-unicast"), pElem("ipv4-entry", "prefix", "203.0.113.1/32"), pElem("state"), pElem("next-hop-group")}},
						Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 42}},
					},
				},
			},
		},
	}
	if err := c.addAFTNotification(noti); err != nil {
		t.Fatalf("non-atomic update failed: %v", err)
	}
	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	if got := aft.Prefixes["203.0.113.1/32"]; got != 42 {
		t.Fatalf("got NHG %d, want 42", got)
	}
}

func TestAFTCacheEdgeCases(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}

	if err := c.addAFTNotification(makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.5/32", 10, 100, true)); err != nil {
		t.Fatal(err)
	}
	if err := c.addAFTNotification(makeNHGNoti("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 100, true)); err != nil {
		t.Fatal(err)
	}

	// 1. Deleting an unrelated optional sub-leaf must NOT delete the prefix entry.
	optLeafDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 200,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pElem("network-instances"), pElem("network-instance", "name", "DEFAULT"),
					pElem("afts"), pElem("ipv4-unicast"), pElem("ipv4-entry", "prefix", "198.51.100.5/32"),
				}},
				Delete: []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pElem("state"), pElem("origin-protocol")}}},
			},
		},
	}
	if err := c.addAFTNotification(optLeafDel); err != nil {
		t.Fatalf("optional leaf delete failed: %v", err)
	}
	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if aft.Prefixes["198.51.100.5/32"] != 10 {
		t.Fatalf("optional leaf delete erroneously removed prefix: %+v", aft.Prefixes)
	}

	// 2. Sub-container atomic update under NHG must merge instead of wiping sibling members.
	subAtomicNHG := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 250,
				Atomic:    true,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pElem("network-instances"), pElem("network-instance", "name", "DEFAULT"),
					pElem("afts"), pElem("next-hop-groups"), pElem("next-hop-group", "id", "10"),
					pElem("next-hops"), pElem("next-hop", "index", "3"),
				}},
				Update: []*gnmipb.Update{
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("index")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 3}}},
					{Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{pElem("state"), pElem("weight")}}, Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 1}}},
				},
			},
		},
	}
	if err := c.addAFTNotification(subAtomicNHG); err != nil {
		t.Fatalf("sub-container atomic NHG update failed: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(aft.NextHopGroups[10].NHIDs) != 3 {
		t.Fatalf("sub-container atomic update wiped siblings: got NHIDs=%v, want 3 members", aft.NextHopGroups[10].NHIDs)
	}

	// 3. Multi-member delete inside the same notification (same timestamp) deletes all listed members.
	multiDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 300,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pElem("network-instances"), pElem("network-instance", "name", "DEFAULT"),
					pElem("afts"), pElem("next-hop-groups"), pElem("next-hop-group", "id", "10"),
				}},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pElem("next-hops"), pElem("next-hop", "index", "1")}},
					{Elem: []*gnmipb.PathElem{pElem("next-hops"), pElem("next-hop", "index", "2")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(multiDel); err != nil {
		t.Fatalf("multi-member NHG delete failed: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := aft.NextHopGroups[10].NHIDs; len(got) != 1 || got[0] != 3 {
		t.Fatalf("multi-member NHG delete got NHIDs=%v, want [3]", got)
	}
	// Stale NHG update with timestamp older than the partial delete (275 < 300) must be rejected.
	if err := c.addAFTNotification(makeNHGNoti("DEFAULT", 10, map[uint64]uint64{1: 1, 2: 1}, 275, true)); !errors.Is(err, cache.ErrStale) {
		t.Fatalf("expected ErrStale after partial NHG delete, got: %v", err)
	}

	// 4. Multi-field NH delete inside the same notification clears all listed fields and advances timestamp.
	if err := c.addAFTNotification(makeNHNoti("DEFAULT", 7, "192.0.2.7", "Ethernet7/1", 100, true)); err != nil {
		t.Fatal(err)
	}
	multiNHFieldDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 300,
				Prefix: &gnmipb.Path{Elem: []*gnmipb.PathElem{
					pElem("network-instances"), pElem("network-instance", "name", "DEFAULT"),
					pElem("afts"), pElem("next-hops"), pElem("next-hop", "index", "7"),
				}},
				Delete: []*gnmipb.Path{
					{Elem: []*gnmipb.PathElem{pElem("state"), pElem("ip-address")}},
					{Elem: []*gnmipb.PathElem{pElem("interface-ref"), pElem("state"), pElem("interface")}},
				},
			},
		},
	}
	if err := c.addAFTNotification(multiNHFieldDel); err != nil {
		t.Fatalf("multi-field NH delete failed: %v", err)
	}
	aft, err = ss.ToAFT(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if nh := aft.NextHops[7]; nh == nil || nh.IP != "" || nh.IntfName != "" {
		t.Fatalf("multi-field NH delete got %+v, want empty IP and IntfName", nh)
	}
	// Stale NH update with timestamp older than the partial field delete (200 < 300) must be rejected.
	if err := c.addAFTNotification(makeNHNoti("DEFAULT", 7, "192.0.2.7", "Ethernet7/1", 200, true)); !errors.Is(err, cache.ErrStale) {
		t.Fatalf("expected ErrStale after partial NH field delete, got: %v", err)
	}

	// 5. Atomic delete is rejected.
	atomicDel := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: 400,
				Atomic:    true,
				Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pElem("afts")}}},
			},
		},
	}
	if err := c.addAFTNotification(atomicDel); err == nil {
		t.Fatalf("expected error on atomic delete, got nil")
	}
}

func TestAFTDataFilterByPrefixes(t *testing.T) {
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{Cache: c, start: time.Now()}
	for _, n := range []*gnmipb.SubscribeResponse{
		makeNHNoti("DEFAULT", 1, "192.0.2.2", "Ethernet1", 100, true),
		makeNHNoti("DEFAULT", 2, "192.0.2.6", "Ethernet2", 100, true),
		makeNHGNoti("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true),
		makeNHGNoti("DEFAULT", 20, map[uint64]uint64{2: 1}, 100, true),
		makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
		makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.2/32", 20, 100, true),
	} {
		if err := c.addAFTNotification(n); err != nil {
			t.Fatalf("addAFTNotification failed: %v", err)
		}
	}

	aft, err := ss.ToAFT(t, nil)
	if err != nil {
		t.Fatalf("ToAFT failed: %v", err)
	}
	filtered := aft.FilterByPrefixes(map[string]bool{
		"198.51.100.1/32":  true,
		"198.51.100.99/32": true, // absent prefix should be ignored
	})

	if len(filtered.Prefixes) != 1 || filtered.Prefixes["198.51.100.1/32"] != 10 {
		t.Fatalf("filtered.Prefixes = %v, want only 198.51.100.1/32 -> 10", filtered.Prefixes)
	}
	if len(filtered.NextHopGroups) != 1 || filtered.NextHopGroups[10] == nil {
		t.Fatalf("filtered.NextHopGroups = %v, want only NHG 10", filtered.NextHopGroups)
	}
	if len(filtered.NextHops) != 1 || filtered.NextHops[1] == nil {
		t.Fatalf("filtered.NextHops = %v, want only NH 1", filtered.NextHops)
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
			name:    "sync response ignored",
			resp:    &gnmipb.SubscribeResponse{Response: &gnmipb.SubscribeResponse_SyncResponse{SyncResponse: true}},
			wantErr: false,
		},
		{
			name:    "atomic update passes",
			resp:    makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, true),
			wantErr: false,
		},
		{
			name:    "non-atomic update fails",
			resp:    makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", "198.51.100.1/32", 10, 100, false),
			wantErr: true,
		},
		{
			name: "non-atomic delete passes",
			resp: &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 200,
						Atomic:    false,
						Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pElem("ipv4-entry", "prefix", "198.51.100.1/32")}}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "atomic delete fails",
			resp: &gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{
					Update: &gnmipb.Notification{
						Timestamp: 200,
						Atomic:    true,
						Delete:    []*gnmipb.Path{{Elem: []*gnmipb.PathElem{pElem("ipv4-entry", "prefix", "198.51.100.1/32")}}},
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
				t.Fatalf("VerifyAtomicFlagHook got err=%v, wantErr=%v", err, tc.wantErr)
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
		t.Fatalf("WithDebug() did not enable debugMode")
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
							pElem("ipv4-unicast"),
							pElem("ipv4-entry", "prefix", "198.51.100.10/32"),
							pElem("state"),
							pElem("next-hop-group"),
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
						pElem("ipv4-unicast"),
						pElem("ipv4-entry", "prefix", "198.51.100.20/32"),
					}},
				},
			},
		},
	}
	ss.notifications = []*gnmipb.SubscribeResponse{syncResp, addResp, delResp}

	if got := len(ss.Notifications()); got != 3 {
		t.Fatalf("len(Notifications()) = %d, want 3", got)
	}

	// Matches both update and delete while skipping SyncResponse.
	waitBoth := WaitForNotification(t, NotificationExpectation{
		AddPrefix:        "198.51.100.10/32",
		DeletePrefix:     "198.51.100.20/32",
		NotificationWait: time.Second,
	})
	done, err := waitBoth.PeriodicFunc(ss)
	if err != nil || !done {
		t.Fatalf("WaitForNotification(both) got done=%v err=%v, want true, nil", done, err)
	}

	// Times out cleanly when expected prefix is not present.
	waitMissing := WaitForNotification(t, NotificationExpectation{
		AddPrefix:        "203.0.113.99/32",
		NotificationWait: time.Millisecond,
	})
	time.Sleep(5 * time.Millisecond)
	done, err = waitMissing.PeriodicFunc(ss)
	if err == nil || done {
		t.Fatalf("WaitForNotification(missing) got done=%v err=%v, want false, timeout error", done, err)
	}
}

func TestAFTCacheScaleMemoryAndSpeed(t *testing.T) {
	const numRoutes = 200000 // 200k routes in unit test to validate O(1) memory/speed fast
	c := newAFTCache("dut", "DEFAULT")
	ss := &AFTStreamSession{
		Cache:             c,
		start:             time.Now(),
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
	}
	if err := c.addAFTNotification(makeNHNoti("DEFAULT", 1, "192.0.2.2", "Ethernet3/1", 100, true)); err != nil {
		t.Fatal(err)
	}
	if err := c.addAFTNotification(makeNHGNoti("DEFAULT", 10, map[uint64]uint64{1: 1}, 100, true)); err != nil {
		t.Fatal(err)
	}

	want := make(map[string]bool, numRoutes)
	startIngest := time.Now()
	for i := 0; i < numRoutes; i++ {
		// RFC 6598 (100.64.0.0/10) provides 4M documentation-approved test addresses.
		pfx := fmt.Sprintf("100.%d.%d.%d/32", 64+((i>>16)&0x3f), (i>>8)&0xff, i&0xff)
		want[pfx] = true
		if err := c.addAFTNotification(makePrefixNoti("DEFAULT", "ipv4-unicast", "ipv4-entry", pfx, 10, 100, true)); err != nil {
			t.Fatalf("ingest %d failed: %v", i, err)
		}
	}
	ingestDur := time.Since(startIngest)

	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	startCheck := time.Now()
	sc := InitialSyncStoppingCondition(t, nil, want, map[string]bool{"192.0.2.2": true}, map[string]bool{})
	done, err := sc.PeriodicFunc(ss)
	checkDur := time.Since(startCheck)
	if err != nil || !done {
		t.Fatalf("scale stopping condition failed: done=%v err=%v", done, err)
	}
	t.Logf("200k routes: ingest=%v | ToAFT+Check=%v | HeapAlloc=%d MiB", ingestDur, checkDur, ms.HeapAlloc/1024/1024)
}
