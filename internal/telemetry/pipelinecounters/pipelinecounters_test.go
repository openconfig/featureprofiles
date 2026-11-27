// Copyright 2026 Google LLC
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

package pipelinecounters

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func vendorElems(component string, rest ...string) []*gpb.PathElem {
	elems := []*gpb.PathElem{
		{Name: "components"},
		{Name: "component", Key: map[string]string{"name": component}},
		{Name: "integrated-circuit"},
		{Name: "pipeline-counters"},
		{Name: "drop"},
		{Name: "vendor"},
	}
	for _, r := range rest {
		elems = append(elems, &gpb.PathElem{Name: r})
	}
	return elems
}

func uintUpdate(elems []*gpb.PathElem, v uint64) *gpb.Update {
	return &gpb.Update{
		Path: &gpb.Path{Elem: elems},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_UintVal{UintVal: v}},
	}
}

func TestParseLeaf(t *testing.T) {
	tests := []struct {
		desc   string
		elems  []*gpb.PathElem
		want   LeafKey
		wantOK bool
	}{
		{
			desc:   "grouped packet-processing leaf",
			elems:  vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE"),
			want:   LeafKey{Component: "NPU0", RelPath: "Acme/asic1/packet-processing/state/NULL_ROUTE", Class: ClassPacketProcessing},
			wantOK: true,
		},
		{
			desc:   "grouped adverse leaf",
			elems:  vendorElems("NPU1", "Acme", "asic1", "adverse", "state", "PARITY"),
			want:   LeafKey{Component: "NPU1", RelPath: "Acme/asic1/adverse/state/PARITY", Class: ClassAdverse},
			wantOK: true,
		},
		{
			desc:   "ungrouped leaf",
			elems:  vendorElems("NPU0", "Acme", "asic1", "state", "DROP_X"),
			want:   LeafKey{Component: "NPU0", RelPath: "Acme/asic1/state/DROP_X"},
			wantOK: true,
		},
		{
			desc:  "vendor container only",
			elems: vendorElems("NPU0"),
		},
		{
			desc: "keyed list entry keeps its key",
			elems: append(vendorElems("FPC0:NPU0", "juniper", "bt-asic", "packet-processing", "state"),
				&gpb.PathElem{Name: "trap", Key: map[string]string{"name": "dlu-fed-dis"}},
				&gpb.PathElem{Name: "count"}),
			want:   LeafKey{Component: "FPC0:NPU0", RelPath: "juniper/bt-asic/packet-processing/state/trap[name=dlu-fed-dis]/count", Class: ClassPacketProcessing},
			wantOK: true,
		},
		{
			desc: "not a vendor drop path",
			elems: []*gpb.PathElem{
				{Name: "components"},
				{Name: "component", Key: map[string]string{"name": "NPU0"}},
				{Name: "state"},
				{Name: "name"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, ok := parseLeaf(tc.elems)
			if ok != tc.wantOK {
				t.Fatalf("parseLeaf() ok = %t, want %t", ok, tc.wantOK)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("parseLeaf() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyAndHelpers(t *testing.T) {
	s := newStream()
	pp := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	pp1 := vendorElems("NPU1", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	adv := vendorElems("NPU0", "Acme", "asic1", "adverse", "state", "PARITY")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 10), uintUpdate(pp1, 5), uintUpdate(adv, 0)}})
	base := s.Latest()
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 2010), uintUpdate(pp1, 4005), uintUpdate(adv, 0)}})
	d := Deltas(base, s.Latest())

	wantDeltas := Snapshot{
		{Component: "NPU0", RelPath: "Acme/asic1/packet-processing/state/NULL_ROUTE", Class: ClassPacketProcessing}: 2000,
		{Component: "NPU1", RelPath: "Acme/asic1/packet-processing/state/NULL_ROUTE", Class: ClassPacketProcessing}: 4000,
	}
	if diff := cmp.Diff(wantDeltas, d); diff != "" {
		t.Errorf("Deltas() returned diff (-want +got):\n%s", diff)
	}
	if got, want := IngressSum(d, []string{"NPU0"})["Acme/asic1/packet-processing/state/NULL_ROUTE"], uint64(2000); got != want {
		t.Errorf("IngressSum(NPU0) = %d, want %d", got, want)
	}
	all := IngressSum(d, nil)
	if got, want := all["Acme/asic1/packet-processing/state/NULL_ROUTE"], uint64(6000); got != want {
		t.Errorf("IngressSum(all) = %d, want %d", got, want)
	}
	if diff := cmp.Diff([]string{"Acme/asic1/packet-processing/state/NULL_ROUTE"}, LeavesWithSum(all, 6000)); diff != "" {
		t.Errorf("LeavesWithSum() returned diff (-want +got):\n%s", diff)
	}
	if got := ClassDelta(d, ClassAdverse); got != 0 {
		t.Errorf("ClassDelta(adverse) = %d, want 0", got)
	}
	if got, want := ClassDelta(d, ClassPacketProcessing), uint64(6000); got != want {
		t.Errorf("ClassDelta(packet-processing) = %d, want %d", got, want)
	}
	if got := s.MinUpdates(); got != 2 {
		t.Errorf("MinUpdates() = %d, want 2", got)
	}
}

func TestMonotonicAndNonCounter(t *testing.T) {
	s := newStream()
	pp := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	str := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "DESCRIPTION")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 100)}})
	s.apply(&gpb.Notification{Update: []*gpb.Update{
		uintUpdate(pp, 50),
		{Path: &gpb.Path{Elem: str}, Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "x"}}},
	}})
	if got := len(s.MonotonicViolations()); got != 1 {
		t.Errorf("MonotonicViolations() returned %d entries, want 1", got)
	}
	if diff := cmp.Diff([]string{"NPU0:Acme/asic1/packet-processing/state/DESCRIPTION"}, s.NonCounterLeaves()); diff != "" {
		t.Errorf("NonCounterLeaves() returned diff (-want +got):\n%s", diff)
	}
}

func TestKeyedListEntries(t *testing.T) {
	s := newStream()
	trap := func(name, leaf string) []*gpb.PathElem {
		return append(vendorElems("FPC0:NPU0", "juniper", "bt-asic", "packet-processing", "state"),
			&gpb.PathElem{Name: "trap", Key: map[string]string{"name": name}},
			&gpb.PathElem{Name: leaf})
	}
	s.apply(&gpb.Notification{Update: []*gpb.Update{
		uintUpdate(trap("a", "count"), 35),
		uintUpdate(trap("b", "count"), 0),
		{Path: &gpb.Path{Elem: trap("a", "name")}, Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "a"}}},
	}})
	if got := len(s.Latest()); got != 2 {
		t.Errorf("Latest() returned %d leaves, want 2 distinct list entries", got)
	}
	if got := s.MonotonicViolations(); len(got) != 0 {
		t.Errorf("MonotonicViolations() = %v, want none (list entries must not overwrite each other)", got)
	}
	if got := s.NonCounterLeaves(); len(got) != 0 {
		t.Errorf("NonCounterLeaves() = %v, want none (key echo leaves are ignored)", got)
	}
}

func TestRateLeavesExcluded(t *testing.T) {
	s := newStream()
	trap := func(leaf string) []*gpb.PathElem {
		return append(vendorElems("FPC0:NPU0", "juniper", "bt-asic", "packet-processing", "state"),
			&gpb.PathElem{Name: "trap", Key: map[string]string{"name": "a"}},
			&gpb.PathElem{Name: leaf})
	}
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(trap("count"), 10), uintUpdate(trap("rate-pps"), 100)}})
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(trap("count"), 20), uintUpdate(trap("rate-pps"), 2)}})
	want := Snapshot{{Component: "FPC0:NPU0", RelPath: "juniper/bt-asic/packet-processing/state/trap[name=a]/count", Class: ClassPacketProcessing}: 20}
	if diff := cmp.Diff(want, s.Latest()); diff != "" {
		t.Errorf("Latest() returned diff (-want +got):\n%s", diff)
	}
	if got := s.MonotonicViolations(); len(got) != 0 {
		t.Errorf("MonotonicViolations() = %v, want none (rate leaves are not counters)", got)
	}
	if diff := cmp.Diff([]string{"FPC0:NPU0:juniper/bt-asic/packet-processing/state/trap[name=a]/rate-pps"}, s.RateLeaves()); diff != "" {
		t.Errorf("RateLeaves() returned diff (-want +got):\n%s", diff)
	}
}

func TestIsRateLeaf(t *testing.T) {
	for name, want := range map[string]bool{"rate": true, "rate-pps": true, "drop-rate": true, "count": false, "NULL_ROUTE": false, "ACCURATE_DROPS": false} {
		if got := isRateLeaf([]*gpb.PathElem{{Name: name}}); got != want {
			t.Errorf("isRateLeaf(%q) = %t, want %t", name, got, want)
		}
	}
}

func TestAwaitRound(t *testing.T) {
	s := newStream()
	pp := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 1)}})
	go s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 2)}})
	snap, ok, err := s.AwaitRound(5 * time.Second)
	if err != nil || !ok {
		t.Fatalf("AwaitRound() = ok %t, err %v; want ok true, err nil", ok, err)
	}
	if got := snap[LeafKey{Component: "NPU0", RelPath: "Acme/asic1/packet-processing/state/NULL_ROUTE", Class: ClassPacketProcessing}]; got != 2 {
		t.Errorf("AwaitRound() value = %d, want 2", got)
	}
	if _, ok, _ := s.AwaitRound(10 * time.Millisecond); ok {
		t.Errorf("AwaitRound() with no new updates = ok true, want false")
	}
}

// TestAwaitRoundChangedOnly covers devices that only resend counters whose
// value changed: the round completes once 1.5 sample intervals have passed and
// a new notification arrived, even if other leaves were not resent. The new
// notification arrives before 1.5 intervals have passed and no other one
// follows, so the round must complete at 1.5 intervals, not at the timeout.
func TestAwaitRoundChangedOnly(t *testing.T) {
	s := newStream()
	s.interval = 50 * time.Millisecond
	a := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "A")
	b := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "B")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(a, 1), uintUpdate(b, 7)}})
	time.AfterFunc(10*time.Millisecond, func() {
		s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(a, 2)}})
	})
	start := time.Now()
	snap, ok, err := s.AwaitRound(5 * time.Second)
	if err != nil || !ok {
		t.Fatalf("AwaitRound() = ok %t, err %v; want ok true, err nil", ok, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("AwaitRound() took %v, want about 75ms (1.5 sample intervals), well below the 5s timeout", elapsed)
	}
	if got := snap[LeafKey{Component: "NPU0", RelPath: "Acme/asic1/packet-processing/state/B", Class: ClassPacketProcessing}]; got != 7 {
		t.Errorf("AwaitRound() value of the leaf that was not resent = %d, want 7 (last value kept)", got)
	}
}

func TestMergePrefix(t *testing.T) {
	n := &gpb.Notification{
		Prefix: &gpb.Path{Target: "dut", Origin: "native", Elem: []*gpb.PathElem{{Name: "a"}}},
		Update: []*gpb.Update{uintUpdate([]*gpb.PathElem{{Name: "b"}}, 7)},
	}
	got := mergePrefix(n)
	if got.GetPrefix().GetTarget() != "dut" || len(got.GetPrefix().GetElem()) != 0 {
		t.Errorf("mergePrefix() prefix = %v, want target only", got.GetPrefix())
	}
	p := got.GetUpdate()[0].GetPath()
	if p.GetOrigin() != "native" || len(p.GetElem()) != 2 {
		t.Errorf("mergePrefix() path = %v, want origin native with 2 elems", p)
	}
}

func TestAggregateDelta(t *testing.T) {
	u := func(v uint64) *uint64 { return &v }
	base := map[string]Aggregates{"NPU0": {PacketProcessing: u(10)}, "NPU1": {PacketProcessing: u(0)}}
	final := map[string]Aggregates{"NPU0": {PacketProcessing: u(1010)}, "NPU1": {PacketProcessing: u(500)}}
	pp := func(a Aggregates) *uint64 { return a.PacketProcessing }
	got, err := AggregateDelta(base, final, []string{"NPU0", "NPU1"}, pp)
	if err != nil || got != 1500 {
		t.Errorf("AggregateDelta() = %d, %v; want 1500, nil", got, err)
	}
	adv := func(a Aggregates) *uint64 { return a.Adverse }
	if _, err := AggregateDelta(base, final, []string{"NPU0"}, adv); !errors.Is(err, ErrNotReported) {
		t.Errorf("AggregateDelta(missing) err = %v, want ErrNotReported", err)
	}
}

func TestJSONScalarCounter(t *testing.T) {
	for _, in := range []string{`123`, `"123"`, ` 123 `} {
		if got, ok := jsonScalarCounter([]byte(in)); !ok || got != 123 {
			t.Errorf("jsonScalarCounter(%q) = %d, %t; want 123, true", in, got, ok)
		}
	}
	if _, ok := jsonScalarCounter([]byte(`{"a":1}`)); ok {
		t.Errorf("jsonScalarCounter(object) ok = true, want false")
	}
}

func TestLeavesExceedingGapStopped(t *testing.T) {
	s := newStream()
	pp := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 1)}})
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 2)}})
	if got := s.LeavesExceedingGap(time.Minute); len(got) != 0 {
		t.Errorf("LeavesExceedingGap(1m) right after two updates = %v, want none", got)
	}
	for k := range s.lastSeen {
		s.lastSeen[k] = time.Now().Add(-time.Hour)
	}
	if got := s.LeavesExceedingGap(time.Minute); len(got) != 1 {
		t.Errorf("LeavesExceedingGap(1m) for a leaf last updated 1h ago = %v, want 1 entry (stopped streaming)", got)
	}
}

func TestDiff(t *testing.T) {
	a := LeafKey{Component: "NPU0", RelPath: "x"}
	b := LeafKey{Component: "NPU0", RelPath: "y"}
	want := Snapshot{a: 1, b: 2}
	if got := Diff(want, Snapshot{a: 1, b: 2}); len(got) != 0 {
		t.Errorf("Diff(equal) = %v, want empty", got)
	}
	if got := Diff(want, Snapshot{a: 5}); len(got) != 2 {
		t.Errorf("Diff(changed+missing) = %v, want 2 entries", got)
	}
}

func TestParseAggregate(t *testing.T) {
	base := []*gpb.PathElem{
		{Name: "components"},
		{Name: "component", Key: map[string]string{"name": "NPU0"}},
		{Name: "integrated-circuit"},
		{Name: "pipeline-counters"},
		{Name: "drop"},
	}
	with := func(rest ...string) []*gpb.PathElem {
		e := append([]*gpb.PathElem{}, base...)
		for _, r := range rest {
			e = append(e, &gpb.PathElem{Name: r})
		}
		return e
	}
	for _, tc := range []struct {
		elems  []*gpb.PathElem
		want   LeafKey
		wantOK bool
	}{
		{with("state", "adverse-aggregate"), LeafKey{Component: "NPU0", RelPath: "state/adverse-aggregate", Class: ClassOC}, true},
		{with("lookup-block", "state", "no-route"), LeafKey{Component: "NPU0", RelPath: "lookup-block/state/no-route", Class: ClassOC}, true},
		{with("interface-block", "state", "in-drops"), LeafKey{}, false},
		{with("vendor", "Acme", "state", "X"), LeafKey{}, false},
	} {
		got, ok := parseAggregate(tc.elems)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("parseAggregate(%v) = %v, %t; want %v, %t", tc.elems, got, ok, tc.want, tc.wantOK)
		}
	}
	s := newStream()
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(with("state", "packet-processing-aggregate"), 42)}})
	aggs := AggregatesFrom(s.Latest(), []string{"NPU0"})
	if p := aggs["NPU0"].PacketProcessing; p == nil || *p != 42 {
		t.Errorf("AggregatesFrom().PacketProcessing = %v, want 42", p)
	}
	if got := len(s.Leaves()); got != 0 {
		t.Errorf("Leaves() returned %d vendor leaves for an aggregate-only stream, want 0", got)
	}
	if got := IngressSum(s.Latest(), nil); len(got) != 0 {
		t.Errorf("IngressSum() included ClassOC leaves: %v", got)
	}
}

func TestEmptyRoundAndGap(t *testing.T) {
	s := newStream()
	if _, ok, err := s.AwaitRound(10 * time.Millisecond); err != nil || !ok {
		t.Errorf("AwaitRound() on an empty stream = %t, %v; want true, nil", ok, err)
	}
	pp := vendorElems("NPU0", "Acme", "asic1", "packet-processing", "state", "NULL_ROUTE")
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 1)}})
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(pp, 2)}})
	if got := s.LeavesExceedingGap(time.Hour); len(got) != 0 {
		t.Errorf("LeavesExceedingGap(1h) = %v, want none", got)
	}
	if got := s.LeavesExceedingGap(-1); len(got) != 1 {
		t.Errorf("LeavesExceedingGap(-1) = %v, want 1 entry", got)
	}
	agg := []*gpb.PathElem{
		{Name: "components"},
		{Name: "component", Key: map[string]string{"name": "NPU0"}},
		{Name: "integrated-circuit"},
		{Name: "pipeline-counters"},
		{Name: "drop"},
		{Name: "state"},
		{Name: "adverse-aggregate"},
	}
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(agg, 1)}})
	s.apply(&gpb.Notification{Update: []*gpb.Update{uintUpdate(agg, 1)}})
	if got := s.LeavesExceedingGap(-1); len(got) != 1 {
		t.Errorf("LeavesExceedingGap(-1) with an OC leaf = %v, want only the vendor leaf", got)
	}
}

func TestAggregateDeltaDecrease(t *testing.T) {
	u := func(v uint64) *uint64 { return &v }
	base := map[string]Aggregates{"NPU0": {Adverse: u(10)}}
	final := map[string]Aggregates{"NPU0": {Adverse: u(5)}}
	if _, err := AggregateDelta(base, final, []string{"NPU0"}, func(a Aggregates) *uint64 { return a.Adverse }); !errors.Is(err, ErrDecreased) {
		t.Errorf("AggregateDelta(decrease) err = %v, want ErrDecreased", err)
	}
}
