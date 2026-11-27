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

// Package pipelinecounters provides helpers to stream and validate the
// vendor-defined drop counters under
// /components/component/integrated-circuit/pipeline-counters/drop/vendor and
// the OpenConfig drop aggregates of INTEGRATED_CIRCUIT components.
//
// The vendor container is augmented by each vendor, so it has no generated
// ygot leaves. Stream therefore uses a raw gNMI STREAM/SAMPLE subscription and
// identifies every counter by its path below .../drop/vendor. Devices that only
// expose these counters through a functional translator (for example Cisco XR
// via the ciscoxr_vendordrop_ft deviation) are supported by subscribing to the
// translator's native input paths and translating each notification.
package pipelinecounters

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// Drop counter classes defined by the OpenConfig vendor counter guide.
const (
	ClassAdverse          = "adverse"
	ClassCongestion       = "congestion"
	ClassPacketProcessing = "packet-processing"
	// ClassOC marks the OpenConfig drop aggregates and lookup block counters
	// (paths .../drop/state/* and .../drop/lookup-block/state/*). Leaves of
	// this class are tracked by Stream but excluded from vendor leaf helpers.
	ClassOC = "oc"
)

// DefaultSampleInterval is the SAMPLE interval used when Options does not set
// one.
const DefaultSampleInterval = 10 * time.Second

// LeafKey identifies one vendor drop counter on one integrated circuit.
type LeafKey struct {
	// Component is the INTEGRATED_CIRCUIT component name.
	Component string
	// RelPath is the path below .../pipeline-counters/drop/vendor, for example
	// "<VENDOR>/<PLATFORM>/packet-processing/state/<REASON>".
	RelPath string
	// Class is "adverse", "congestion" or "packet-processing" when the leaf is
	// grouped as recommended by the OpenConfig vendor counter guide, ClassOC
	// for an OpenConfig aggregate or lookup block counter, else "".
	Class string
}

// FullPath returns the full OpenConfig path of the leaf.
func (k LeafKey) FullPath() string {
	base := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/drop", k.Component)
	if k.Class == ClassOC {
		return base + "/" + k.RelPath
	}
	return base + "/vendor/" + k.RelPath
}

// String returns "<component>:<relative path>".
func (k LeafKey) String() string {
	return fmt.Sprintf("%s:%s", k.Component, k.RelPath)
}

// Snapshot maps every vendor drop leaf to its counter value.
type Snapshot map[LeafKey]uint64

func (s Snapshot) clone() Snapshot {
	c := make(Snapshot, len(s))
	for k, v := range s {
		c[k] = v
	}
	return c
}

// Options configures a Stream.
type Options struct {
	// SampleInterval is the gNMI SAMPLE interval. Defaults to
	// DefaultSampleInterval.
	SampleInterval time.Duration
	// FunctionalTranslator is the name of a registered functional translator
	// (for example deviations.CiscoxrVendordropFt(dut)). When empty, the native
	// OpenConfig vendor drop path is subscribed to directly.
	FunctionalTranslator string
}

// functionalTranslator is the subset of the functional translator API used by
// this package.
type functionalTranslator interface {
	Translate(*gpb.SubscribeResponse) (*gpb.SubscribeResponse, error)
	OutputToInputMap() map[string][]*gpb.Path
}

// Stream holds the latest value of every vendor drop leaf received from a
// STREAM/SAMPLE subscription.
type Stream struct {
	mu         sync.Mutex
	values     Snapshot
	updates    map[LeafKey]int
	lastSeen   map[LeafKey]time.Time
	maxGap     map[LeafKey]time.Duration
	nonCounter map[string]bool
	rates      map[string]bool
	violations []string
	ftErrors   int
	synced     bool
	err        error
	closed     bool
	changed    chan struct{}
	cancel     context.CancelFunc
	ft         functionalTranslator
	// interval is the requested sample interval. It is 0 in unit tests that
	// build a Stream without a subscription.
	interval time.Duration
	// notifs counts the notifications that carried at least one drop counter.
	notifs int
}

func newStream() *Stream {
	return &Stream{
		values:     Snapshot{},
		updates:    map[LeafKey]int{},
		lastSeen:   map[LeafKey]time.Time{},
		maxGap:     map[LeafKey]time.Duration{},
		nonCounter: map[string]bool{},
		rates:      map[string]bool{},
		changed:    make(chan struct{}),
	}
}

// VendorDropPath returns the wildcard OpenConfig path
// /components/component[name=*]/integrated-circuit/pipeline-counters/drop/vendor.
func VendorDropPath() *gpb.Path {
	return &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "components"},
			{Name: "component", Key: map[string]string{"name": "*"}},
			{Name: "integrated-circuit"},
			{Name: "pipeline-counters"},
			{Name: "drop"},
			{Name: "vendor"},
		},
	}
}

// dropPath returns the wildcard OpenConfig path
// /components/component[name=*]/integrated-circuit/pipeline-counters/drop/<rest>.
func dropPath(rest ...string) *gpb.Path {
	p := VendorDropPath()
	p.Elem = p.Elem[:len(p.Elem)-1]
	for _, r := range rest {
		p.Elem = append(p.Elem, &gpb.PathElem{Name: r})
	}
	return p
}

// aggregatePaths returns the wildcard OpenConfig paths of the drop aggregates
// (.../drop/state) and lookup block counters (.../drop/lookup-block/state).
func aggregatePaths() []*gpb.Path {
	return []*gpb.Path{dropPath("state"), dropPath("lookup-block", "state")}
}

// NewStream opens a gNMI STREAM subscription in SAMPLE mode for the vendor
// drop counters (primary) and the OpenConfig drop aggregates and lookup block
// counters (secondary) of every integrated circuit of the DUT, in a single
// SubscribeRequest. The subscription stays open until Close is called.
func NewStream(t testing.TB, dut *ondatra.DUTDevice, o Options) *Stream {
	t.Helper()
	if o.SampleInterval == 0 {
		o.SampleInterval = DefaultSampleInterval
	}
	s := newStream()
	s.interval = o.SampleInterval
	paths := []*gpb.Path{VendorDropPath()}
	if o.FunctionalTranslator != "" {
		ft, ok := registrar.FunctionalTranslatorRegistry[o.FunctionalTranslator]
		if !ok {
			t.Fatalf("Functional translator %q is not registered", o.FunctionalTranslator)
		}
		s.ft = ft
		paths = ftInputPaths(ft)
		if len(paths) == 0 {
			t.Fatalf("Functional translator %q has no input paths", o.FunctionalTranslator)
		}
	}
	paths = append(paths, aggregatePaths()...)
	var subs []*gpb.Subscription
	for _, p := range paths {
		subs = append(subs, &gpb.Subscription{
			Path:           p,
			Mode:           gpb.SubscriptionMode_SAMPLE,
			SampleInterval: uint64(o.SampleInterval.Nanoseconds()),
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	sub, err := dut.RawAPIs().GNMI(t).Subscribe(ctx)
	if err != nil {
		cancel()
		t.Fatalf("Failed to open gNMI Subscribe stream on %s: %v", dut.Name(), err)
	}
	req := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Prefix:       &gpb.Path{Target: dut.Name()},
				Mode:         gpb.SubscriptionList_STREAM,
				Encoding:     gpb.Encoding_PROTO,
				Subscription: subs,
			},
		},
	}
	if err := sub.Send(req); err != nil {
		cancel()
		t.Fatalf("Failed to send gNMI SubscribeRequest on %s: %v", dut.Name(), err)
	}
	t.Logf("Opened STREAM/SAMPLE (%v) subscription for %d drop counter path(s) on %s", o.SampleInterval, len(paths), dut.Name())
	go s.receive(sub)
	return s
}

func ftInputPaths(ft functionalTranslator) []*gpb.Path {
	seen := map[string]bool{}
	var paths []*gpb.Path
	for _, in := range ft.OutputToInputMap() {
		for _, p := range in {
			key := p.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			paths = append(paths, p)
		}
	}
	return paths
}

// Close cancels the subscription.
func (s *Stream) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Stream) receive(sub gpb.GNMI_SubscribeClient) {
	for {
		resp, err := sub.Recv()
		if err != nil {
			s.mu.Lock()
			if !s.closed {
				s.err = err
			}
			s.notifyLocked()
			s.mu.Unlock()
			return
		}
		switch r := resp.GetResponse().(type) {
		case *gpb.SubscribeResponse_SyncResponse:
			s.mu.Lock()
			s.synced = true
			s.notifyLocked()
			s.mu.Unlock()
		case *gpb.SubscribeResponse_Update:
			s.handleNotification(r.Update)
		}
	}
}

func (s *Stream) handleNotification(n *gpb.Notification) {
	if s.ft != nil && !isOCNotification(n) {
		out, err := s.ft.Translate(&gpb.SubscribeResponse{
			Response: &gpb.SubscribeResponse_Update{Update: mergePrefix(n)},
		})
		if err != nil {
			s.mu.Lock()
			s.ftErrors++
			s.mu.Unlock()
			return
		}
		if out.GetUpdate() == nil {
			return
		}
		n = out.GetUpdate()
	}
	s.apply(n)
}

// isOCNotification reports whether n carries OpenConfig pipeline counter
// paths, which must not be passed to a functional translator.
func isOCNotification(n *gpb.Notification) bool {
	for _, e := range n.GetPrefix().GetElem() {
		if e.GetName() == "pipeline-counters" {
			return true
		}
	}
	for _, u := range n.GetUpdate() {
		for _, e := range u.GetPath().GetElem() {
			if e.GetName() == "pipeline-counters" {
				return true
			}
		}
	}
	return false
}

// mergePrefix returns a copy of n with the prefix elements and origin joined
// into every update path, keeping only the target in the prefix. Functional
// translators expect fully qualified native paths.
func mergePrefix(n *gpb.Notification) *gpb.Notification {
	prefix := n.GetPrefix()
	out := &gpb.Notification{
		Timestamp: n.GetTimestamp(),
		Prefix:    &gpb.Path{Target: prefix.GetTarget()},
	}
	for _, u := range n.GetUpdate() {
		origin := u.GetPath().GetOrigin()
		if origin == "" {
			origin = prefix.GetOrigin()
		}
		elems := append(append([]*gpb.PathElem{}, prefix.GetElem()...), u.GetPath().GetElem()...)
		out.Update = append(out.Update, &gpb.Update{
			Path: &gpb.Path{Origin: origin, Elem: elems},
			Val:  u.GetVal(),
		})
	}
	return out
}

// apply records every vendor drop leaf of n.
func (s *Stream) apply(n *gpb.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counted := false
	for _, u := range n.GetUpdate() {
		elems := append(append([]*gpb.PathElem{}, n.GetPrefix().GetElem()...), u.GetPath().GetElem()...)
		key, ok := parseLeaf(elems)
		if ok && isRateLeaf(elems) {
			// A rate (for example .../trap[name=X]/rate-pps) rises and falls
			// with the traffic; it is not a drop counter.
			s.rates[key.String()] = true
			continue
		}
		if !ok {
			if key, ok = parseAggregate(elems); !ok {
				continue
			}
		}
		val, ok := counterValue(u.GetVal())
		if !ok {
			if isListKeyLeaf(elems) {
				// The leaf echoes the key of its list entry (for example
				// .../trap[name=X]/name); it is not a counter.
				continue
			}
			s.nonCounter[key.String()] = true
			continue
		}
		if old, seen := s.values[key]; seen && val < old {
			s.violations = append(s.violations, fmt.Sprintf("%s decreased from %d to %d", key, old, val))
		}
		now := time.Now()
		if last, seen := s.lastSeen[key]; seen {
			if gap := now.Sub(last); gap > s.maxGap[key] {
				s.maxGap[key] = gap
			}
		}
		s.lastSeen[key] = now
		s.values[key] = val
		s.updates[key]++
		counted = true
	}
	if counted {
		s.notifs++
	}
	s.notifyLocked()
}

func (s *Stream) notifyLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// parseLeaf extracts the integrated circuit name, the path below
// .../drop/vendor and the drop class from a fully qualified path. List keys
// below .../drop/vendor are kept in the relative path (for example
// juniper/bt-asic/packet-processing/state/trap[name=X]/count), so that the
// entries of a keyed vendor list are distinct leaves.
func parseLeaf(elems []*gpb.PathElem) (LeafKey, bool) {
	component := ""
	vendorIdx := -1
	for i, e := range elems {
		if e.GetName() == "component" {
			component = e.GetKey()["name"]
		}
		if e.GetName() == "vendor" && i > 0 && elems[i-1].GetName() == "drop" {
			vendorIdx = i
			break
		}
	}
	if vendorIdx < 0 || vendorIdx == len(elems)-1 || component == "" {
		return LeafKey{}, false
	}
	rest := elems[vendorIdx+1:]
	names := make([]string, len(rest))
	class := ""
	for i, e := range rest {
		names[i] = elemString(e)
		switch e.GetName() {
		case ClassAdverse, ClassCongestion, ClassPacketProcessing:
			if class == "" {
				class = e.GetName()
			}
		}
	}
	return LeafKey{Component: component, RelPath: strings.Join(names, "/"), Class: class}, true
}

// elemString formats a path element with its keys in sorted order, for
// example trap[name=X].
func elemString(e *gpb.PathElem) string {
	if len(e.GetKey()) == 0 {
		return e.GetName()
	}
	keys := make([]string, 0, len(e.GetKey()))
	for k := range e.GetKey() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(e.GetName())
	for _, k := range keys {
		fmt.Fprintf(&b, "[%s=%s]", k, e.GetKey()[k])
	}
	return b.String()
}

// isListKeyLeaf reports whether the last element of elems is a leaf that
// echoes a key of its parent list entry, for example .../trap[name=X]/name.
func isListKeyLeaf(elems []*gpb.PathElem) bool {
	if len(elems) < 2 {
		return false
	}
	_, ok := elems[len(elems)-2].GetKey()[elems[len(elems)-1].GetName()]
	return ok
}

// isRateLeaf reports whether the last element of elems names a rate rather
// than a counter: "rate", or a name that starts with "rate-" (for example
// rate-pps) or ends with "-rate".
func isRateLeaf(elems []*gpb.PathElem) bool {
	if len(elems) == 0 {
		return false
	}
	name := strings.ToLower(elems[len(elems)-1].GetName())
	return name == "rate" || strings.HasPrefix(name, "rate-") || strings.HasSuffix(name, "-rate")
}

// parseAggregate extracts an OpenConfig drop aggregate
// (.../drop/state/<leaf>) or lookup block counter
// (.../drop/lookup-block/state/<leaf>) from a fully qualified path.
func parseAggregate(elems []*gpb.PathElem) (LeafKey, bool) {
	component := ""
	for i, e := range elems {
		if e.GetName() == "component" {
			component = e.GetKey()["name"]
		}
		if e.GetName() != "drop" || i == 0 || elems[i-1].GetName() != "pipeline-counters" || component == "" {
			continue
		}
		var names []string
		for _, r := range elems[i+1:] {
			names = append(names, r.GetName())
		}
		rel := strings.Join(names, "/")
		if (len(names) == 2 && names[0] == "state") || (len(names) == 3 && names[0] == "lookup-block" && names[1] == "state") {
			return LeafKey{Component: component, RelPath: rel, Class: ClassOC}, true
		}
		return LeafKey{}, false
	}
	return LeafKey{}, false
}

func counterValue(v *gpb.TypedValue) (uint64, bool) {
	switch x := v.GetValue().(type) {
	case *gpb.TypedValue_UintVal:
		return x.UintVal, true
	case *gpb.TypedValue_IntVal:
		if x.IntVal >= 0 {
			return uint64(x.IntVal), true
		}
	case *gpb.TypedValue_JsonIetfVal:
		return jsonScalarCounter(x.JsonIetfVal)
	case *gpb.TypedValue_JsonVal:
		return jsonScalarCounter(x.JsonVal)
	}
	return 0, false
}

// jsonScalarCounter parses a JSON-encoded unsigned integer, which may be
// quoted as RFC 7951 requires for 64-bit integers.
func jsonScalarCounter(b []byte) (uint64, bool) {
	v, err := strconv.ParseUint(strings.Trim(strings.TrimSpace(string(b)), `"`), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// waitFor waits until cond, evaluated with the lock held, returns true. It
// returns false when the timeout expires and an error if the subscription
// failed.
func (s *Stream) waitFor(timeout time.Duration, cond func() bool) (bool, error) {
	return s.waitForWake(timeout, time.Time{}, cond)
}

// waitForWake is waitFor that also re-evaluates cond at wake (if non-zero),
// for conditions that become true with the passage of time rather than with a
// new notification.
func (s *Stream) waitForWake(timeout time.Duration, wake time.Time, cond func() bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		if s.err != nil {
			err := s.err
			s.mu.Unlock()
			return false, err
		}
		if cond() {
			s.mu.Unlock()
			return true, nil
		}
		ch := s.changed
		s.mu.Unlock()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, nil
		}
		if untilWake := time.Until(wake); untilWake > 0 && untilWake < remaining {
			remaining = untilWake
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ch:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// AwaitSync waits for the initial sync_response of the subscription.
func (s *Stream) AwaitSync(timeout time.Duration) (bool, error) {
	return s.waitFor(timeout, func() bool { return s.synced })
}

// AwaitRound waits for one complete sample round. A round is complete when
// every leaf known when the call started has been updated at least once more,
// or, for devices that only resend counters whose value changed, when 1.5
// sample intervals have passed and at least one new notification with drop
// counters arrived (leaves that were not resent keep their last value). If no
// leaf is known, the round completes immediately. It returns a copy of the
// latest values.
func (s *Stream) AwaitRound(timeout time.Duration) (Snapshot, bool, error) {
	snap, missing, err := s.awaitRound(timeout)
	return snap, len(missing) == 0 && err == nil, err
}

// awaitRound implements AwaitRound and returns the leaves that were not
// updated when the round timed out (nil if the round completed).
func (s *Stream) awaitRound(timeout time.Duration) (Snapshot, []LeafKey, error) {
	s.mu.Lock()
	start := make(map[LeafKey]int, len(s.updates))
	for k, v := range s.updates {
		start[k] = v
	}
	startNotifs := s.notifs
	s.mu.Unlock()
	begin := time.Now()
	minRound := s.interval * 3 / 2
	var wake time.Time
	if s.interval > 0 {
		wake = begin.Add(minRound)
	}
	ok, err := s.waitForWake(timeout, wake, func() bool {
		if len(start) == 0 {
			// No counter is known: the device reports none, so the round is
			// trivially complete.
			return true
		}
		if s.interval > 0 && s.notifs > startNotifs && time.Since(begin) >= minRound {
			return true
		}
		for k, v := range start {
			if s.updates[k] <= v {
				return false
			}
		}
		return true
	})
	if ok {
		return s.Latest(), nil, err
	}
	s.mu.Lock()
	var missing []LeafKey
	for k, v := range start {
		if s.updates[k] <= v {
			missing = append(missing, k)
		}
	}
	s.mu.Unlock()
	sortKeys(missing)
	return s.Latest(), missing, err
}

// AwaitRounds waits for n consecutive sample rounds and returns the values
// after the last one. It fails the test if a round does not complete, and
// logs how many leaves were updated and which were not.
func (s *Stream) AwaitRounds(t testing.TB, n int, timeout time.Duration) Snapshot {
	t.Helper()
	var snap Snapshot
	for i := 0; i < n; i++ {
		var missing []LeafKey
		var err error
		snap, missing, err = s.awaitRound(timeout)
		if err != nil || len(missing) > 0 {
			s.mu.Lock()
			total := len(s.updates)
			s.mu.Unlock()
			var examples []string
			for j, k := range missing {
				if j == 5 {
					break
				}
				examples = append(examples, k.FullPath())
			}
			t.Fatalf("Vendor drop sample round %d/%d did not complete within %v: err=%v; %d of %d leaves were not updated, for example %v", i+1, n, timeout, err, len(missing), total, examples)
		}
	}
	return snap
}

// AwaitQuietBaseline waits for two consecutive sample rounds in which no drop
// counter leaf (vendor leaves, OC aggregates and lookup block counters)
// changes (increases, decreases or first appears) and returns the second one
// as the baseline. A round pair with changes is retried up to retries times
// before the test fails as "environment not quiescent".
func (s *Stream) AwaitQuietBaseline(t testing.TB, retries int, timeout time.Duration) Snapshot {
	t.Helper()
	for attempt := 0; attempt <= retries; attempt++ {
		first := s.AwaitRounds(t, 1, timeout)
		second := s.AwaitRounds(t, 1, timeout)
		changed := Deltas(first, second)
		other := append(Diff(first, second), Diff(second, first)...)
		if len(changed) == 0 && len(other) == 0 {
			return second
		}
		t.Logf("Drop counters changed between consecutive samples (attempt %d/%d):", attempt+1, retries+1)
		LogIncremented(t, changed)
		if len(changed) == 0 {
			t.Logf("  Changed (decreased, new or missing): %v", other)
		}
	}
	t.Fatalf("Environment not quiescent: drop counters kept changing over %d attempts", retries+1)
	return nil
}

// AwaitIngressSum waits until the ingress sum (see IngressSum) of at least one
// vendor leaf has increased by at least n since base, then waits for one more
// sample round and returns its values. Using the next round catches
// late-arriving over-counting. ok is false if the target is not reached
// within the timeout; the latest values are returned in that case. If the
// target is reached but the next round does not complete, the test fails with
// that reason and the latest values are returned with ok true.
func (s *Stream) AwaitIngressSum(t testing.TB, base Snapshot, ics []string, n uint64, timeout time.Duration) (Snapshot, bool) {
	t.Helper()
	reached, err := s.waitFor(timeout, func() bool {
		return maxValue(IngressSum(Deltas(base, s.values), ics)) >= n
	})
	if err != nil {
		t.Fatalf("Vendor drop subscription failed: %v", err)
	}
	if !reached {
		return s.Latest(), false
	}
	final, ok, err := s.AwaitRound(timeout)
	if err != nil {
		t.Fatalf("Vendor drop subscription failed: %v", err)
	}
	if !ok {
		t.Errorf("A vendor drop leaf reached an ingress sum of %d, but the next sample round did not complete within %v; the latest values are used", n, timeout)
	}
	return final, true
}

// Latest returns a copy of the latest value of every vendor drop leaf.
func (s *Stream) Latest() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values.clone()
}

// Leaves returns all vendor drop leaves (excluding ClassOC) received so far,
// sorted.
func (s *Stream) Leaves() []LeafKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]LeafKey, 0, len(s.values))
	for k := range s.values {
		if k.Class != ClassOC {
			keys = append(keys, k)
		}
	}
	sortKeys(keys)
	return keys
}

// LeavesExceedingGap returns every vendor leaf (ClassOC leaves are excluded)
// whose largest interval between two consecutive updates, or the time since
// its last update (a leaf that stopped streaming), exceeds limit.
func (s *Stream) LeavesExceedingGap(limit time.Duration) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var out []string
	for k, last := range s.lastSeen {
		if k.Class == ClassOC {
			continue
		}
		gap := s.maxGap[k]
		if since := now.Sub(last); since > gap {
			gap = since
		}
		if gap > limit {
			out = append(out, fmt.Sprintf("%s (max interval %v)", k.FullPath(), gap.Round(time.Second)))
		}
	}
	sort.Strings(out)
	return out
}

// UpdateCount returns how many updates were received for a leaf.
func (s *Stream) UpdateCount(k LeafKey) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates[k]
}

// VendorOnly returns the vendor drop leaves of s, excluding ClassOC leaves.
func VendorOnly(s Snapshot) Snapshot {
	out := Snapshot{}
	for k, v := range s {
		if k.Class != ClassOC {
			out[k] = v
		}
	}
	return out
}

// MinUpdates returns the smallest number of updates received for any vendor
// leaf, or 0 if no vendor leaf was received.
func (s *Stream) MinUpdates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	minimum := 0
	first := true
	for k, v := range s.updates {
		if k.Class == ClassOC {
			continue
		}
		if first || v < minimum {
			minimum = v
			first = false
		}
	}
	return minimum
}

// NonCounterLeaves returns the vendor drop leaves whose value is not an
// unsigned integer counter.
func (s *Stream) NonCounterLeaves() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.nonCounter {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RateLeaves returns the vendor drop leaves that report a rate (see
// isRateLeaf). They are not drop counters, so they are excluded from all
// snapshots, round tracking and the monotonicity check.
func (s *Stream) RateLeaves() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.rates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MonotonicViolations returns every observed decrease of a vendor drop
// counter.
func (s *Stream) MonotonicViolations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.violations...)
}

// TranslationErrors returns the number of notifications the functional
// translator failed to translate.
func (s *Stream) TranslationErrors() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ftErrors
}

// Err returns the subscription error, if any.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Deltas returns final - base for every leaf whose value increased. Leaves
// missing from base are treated as starting at 0.
func Deltas(base, final Snapshot) Snapshot {
	d := Snapshot{}
	for k, v := range final {
		if b := base[k]; v > b {
			d[k] = v - b
		}
	}
	return d
}

// IngressSum adds the deltas of each vendor leaf (ClassOC leaves are
// excluded) over the given integrated circuits. The same leaf on different integrated circuits is identified by
// its path below .../drop/vendor (LeafKey.RelPath). If ics is empty, all
// integrated circuits are included.
func IngressSum(d Snapshot, ics []string) map[string]uint64 {
	include := map[string]bool{}
	for _, ic := range ics {
		include[ic] = true
	}
	sums := map[string]uint64{}
	for k, v := range d {
		if k.Class == ClassOC || (len(include) > 0 && !include[k.Component]) {
			continue
		}
		sums[k.RelPath] += v
	}
	return sums
}

// LeavesWithSum returns the relative leaf paths whose sum equals n, sorted.
func LeavesWithSum(sums map[string]uint64, n uint64) []string {
	var out []string
	for k, v := range sums {
		if v == n {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// maxValue returns the largest value in sums, or 0 if sums is empty.
func maxValue(sums map[string]uint64) uint64 {
	var m uint64
	for _, v := range sums {
		if v > m {
			m = v
		}
	}
	return m
}

// ClassDelta returns the sum of the deltas of all leaves of the given class.
func ClassDelta(d Snapshot, class string) uint64 {
	var sum uint64
	for k, v := range d {
		if k.Class == class {
			sum += v
		}
	}
	return sum
}

// LogIncremented logs every leaf with a positive delta, sorted by integrated
// circuit and path.
func LogIncremented(t testing.TB, d Snapshot) {
	t.Helper()
	if len(d) == 0 {
		t.Log("  No vendor drop leaf incremented.")
		return
	}
	keys := make([]LeafKey, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sortKeys(keys)
	for _, k := range keys {
		class := k.Class
		if class == "" {
			class = "ungrouped"
		}
		t.Logf("  Incremented: integrated-circuit=%s leaf=%s class=%s delta=%d", k.Component, k.FullPath(), class, d[k])
	}
}

func sortKeys(keys []LeafKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Component != keys[j].Component {
			return keys[i].Component < keys[j].Component
		}
		return keys[i].RelPath < keys[j].RelPath
	})
}

// Aggregates holds the OpenConfig drop aggregates and lookup block counters of
// one integrated circuit. A nil field means the device does not report it.
type Aggregates struct {
	PacketProcessing *uint64
	Adverse          *uint64
	Congestion       *uint64
	NoRoute          *uint64
	NoNexthop        *uint64
}

// AggregatesFrom builds the Aggregates of each integrated circuit from the
// ClassOC leaves of a Stream snapshot.
func AggregatesFrom(s Snapshot, ics []string) map[string]Aggregates {
	out := make(map[string]Aggregates, len(ics))
	get := func(ic, rel string) *uint64 {
		if v, ok := s[LeafKey{Component: ic, RelPath: rel, Class: ClassOC}]; ok {
			return &v
		}
		return nil
	}
	for _, ic := range ics {
		out[ic] = Aggregates{
			PacketProcessing: get(ic, "state/packet-processing-aggregate"),
			Adverse:          get(ic, "state/adverse-aggregate"),
			Congestion:       get(ic, "state/congestion-aggregate"),
			NoRoute:          get(ic, "lookup-block/state/no-route"),
			NoNexthop:        get(ic, "lookup-block/state/no-nexthop"),
		}
	}
	return out
}

// ErrNotReported is returned by AggregateDelta when a counter is not reported
// by every requested integrated circuit.
var ErrNotReported = errors.New("counter not reported")

// ErrDecreased is returned by AggregateDelta when a counter decreased.
var ErrDecreased = errors.New("counter decreased")

// AggregateDelta returns the sum over ics of field(final) - field(base). It
// returns ErrNotReported if the counter is missing on any integrated circuit
// and ErrDecreased if it decreased on any integrated circuit.
func AggregateDelta(base, final map[string]Aggregates, ics []string, field func(Aggregates) *uint64) (uint64, error) {
	var sum uint64
	for _, ic := range ics {
		b, f := field(base[ic]), field(final[ic])
		if b == nil || f == nil {
			return 0, fmt.Errorf("%s on %s: %w", "aggregate", ic, ErrNotReported)
		}
		if *f < *b {
			return 0, fmt.Errorf("aggregate on %s went from %d to %d: %w", ic, *b, *f, ErrDecreased)
		}
		sum += *f - *b
	}
	return sum, nil
}

// ReadOnce reads the vendor drop counters once with a gNMI Subscribe in ONCE
// mode. When o.FunctionalTranslator is set, the translator's native input
// paths are read and translated.
func ReadOnce(t testing.TB, dut *ondatra.DUTDevice, o Options) (Snapshot, error) {
	t.Helper()
	s := newStream()
	paths := []*gpb.Path{VendorDropPath()}
	if o.FunctionalTranslator != "" {
		ft, ok := registrar.FunctionalTranslatorRegistry[o.FunctionalTranslator]
		if !ok {
			return nil, fmt.Errorf("functional translator %q is not registered", o.FunctionalTranslator)
		}
		s.ft = ft
		paths = ftInputPaths(ft)
	}
	client := dut.RawAPIs().GNMI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	prefix := &gpb.Path{Target: dut.Name()}

	var subs []*gpb.Subscription
	for _, p := range paths {
		subs = append(subs, &gpb.Subscription{Path: p})
	}
	sub, err := client.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("gNMI Subscribe failed: %w", err)
	}
	if err := sub.Send(&gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Prefix:       prefix,
				Mode:         gpb.SubscriptionList_ONCE,
				Encoding:     gpb.Encoding_PROTO,
				Subscription: subs,
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("gNMI Subscribe send failed: %w", err)
	}
	var ns []*gpb.Notification
	for {
		resp, err := sub.Recv()
		if err != nil {
			return nil, fmt.Errorf("gNMI Subscribe ONCE receive failed: %w", err)
		}
		if resp.GetSyncResponse() {
			if err := s.applyOnce(ns); err != nil {
				return nil, err
			}
			return s.Latest(), nil
		}
		if n := resp.GetUpdate(); n != nil {
			ns = append(ns, n)
		}
	}
}

// applyOnce applies the notifications of a single Subscribe ONCE read. With a
// functional translator, all native updates are merged into one notification
// before translation, because translators such as ciscoxr-vendordrops-ft need
// sibling leaves in the same notification. It returns an error if the
// translation fails, so that incomplete data is not reported as a successful
// read.
func (s *Stream) applyOnce(ns []*gpb.Notification) error {
	if s.ft == nil {
		for _, n := range ns {
			s.apply(n)
		}
		return nil
	}
	var native *gpb.Notification
	for _, n := range ns {
		if isOCNotification(n) {
			s.apply(n)
			continue
		}
		m := mergePrefix(n)
		if native == nil {
			native = m
			continue
		}
		native.Update = append(native.Update, m.GetUpdate()...)
	}
	if native != nil {
		s.handleNotification(native)
	}
	if n := s.TranslationErrors(); n != 0 {
		return fmt.Errorf("functional translator failed to translate %d notification(s)", n)
	}
	return nil
}

// Diff returns a description of every leaf of want whose value differs in
// got or is missing from got.
func Diff(want, got Snapshot) []string {
	var out []string
	for k, v := range want {
		g, ok := got[k]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("%s missing", k))
		case g != v:
			out = append(out, fmt.Sprintf("%s: want %d, got %d", k, v, g))
		}
	}
	sort.Strings(out)
	return out
}
