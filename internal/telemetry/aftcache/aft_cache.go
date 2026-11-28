// Copyright 2025 Google LLC
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

// Package aftcache stores a stream of AFT telemetry at full RIB scale in a
// local cache so tests can periodically check convergence conditions, such as
// verifying that all expected prefixes and next hops are present.
//
// Telemetry contract (per OpenConfig openconfig/public#860 and gNMI spec
// Section 2.1.1):
//   - Atomic updates only: ipv4-entry, ipv6-entry, next-hop-group, and
//     next-hop are oc-ext:telemetry-atomic containers. Non-atomic AFT update
//     notifications return an explicit error; atomic notifications carrying
//     deletes are rejected.
//   - Atomic = replace: every AFT entry touched by an atomic notification is
//     replaced as a whole (identified from notification prefix + update path).
//   - Short-prefix atomic updates: atomic notifications without an entry-level
//     prefix are accepted (entries are identified and replaced from full update
//     paths) and counted/logged as compliance warnings.
//   - Deletes: deletes must be non-atomic; any delete at or below an entry key
//     removes the entire entry (table/ancestor deletes remove all matching
//     entries).
//   - Deviation-agnostic: test deviations (such as route scale or expected next
//     hops) are handled by callers rather than inside aftcache.
package aftcache

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/metadata"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

const (
	// periodicInterval is the time between execution of periodic hooks.
	periodicInterval = 2 * time.Minute
	// periodicDeadline is the deadline for all periodic hooks in a run. Should be < periodicInterval.
	periodicDeadline = 1*time.Minute + 45*time.Second
	// aftBufferSize is the capacity of the channel queueing notifications from
	// the DUT before applying them to the cache. Kept at 4M slots
	// (~30 MiB/session) to avoid backpressure regressions on slow-collector and
	// pre-ListenUntil buffering tests.
	aftBufferSize = 4000000
	// missingPrefixesFile is the name of the file where missing prefixes are written.
	missingPrefixesFile = "missing_prefixes.txt"
	// failingNHPrefixesFile is the name of the file where prefixes with failing next hop validation are written.
	failingNHPrefixesFile = "failing_prefixes.txt"
	// notificationsFile is the name of the file where all gNMI notifications are written.
	notificationsFile = "notifications.txt"
)

var (
	// ErrNotExist is an error returned when expected AFT elements are not found and so AFT is inconsistent.
	ErrNotExist = errors.New("does not exist")
	// ErrUnsupported is an error returned when AFT elements are not supported.
	ErrUnsupported = errors.New("unsupported")
	// errNonAtomicUpdate is returned when a non-atomic update targets an atomic
	// AFT container.
	errNonAtomicUpdate = errors.New("non-atomic AFT update (gNMI spec Section 2.1.1)")
)

func subscriptionPaths(dut *ondatra.DUTDevice) map[string][]string {
	defaultNetworkInstance := deviations.DefaultNetworkInstance(dut)
	return map[string][]string{
		"prefix": {
			fmt.Sprintf("network-instances/network-instance[name=%s]/afts/ipv4-unicast/ipv4-entry", defaultNetworkInstance),
			fmt.Sprintf("network-instances/network-instance[name=%s]/afts/ipv6-unicast/ipv6-entry", defaultNetworkInstance),
		},
		"nhg": {
			fmt.Sprintf("network-instances/network-instance[name=%s]/afts/next-hop-groups/next-hop-group", defaultNetworkInstance),
		},
		"nh": {
			fmt.Sprintf("network-instances/network-instance[name=%s]/afts/next-hops/next-hop", defaultNetworkInstance),
		},
	}
}

// AFTData represents an AFT and provides methods for resolving routes.
type AFTData struct {
	// Prefixes contains a map of prefixes to their corresponding next hop group IDs.
	Prefixes map[string]uint64
	// NextHopGroups contains a map of next hop group IDs to their corresponding next hop group data.
	NextHopGroups map[uint64]*aftNextHopGroup
	// NextHops contains a map of next hop IDs to their corresponding next hop data.
	NextHops map[uint64]*aftNextHop
}

// FilterByPrefixes returns a new AFTData containing only the specified prefixes
// and their associated NextHopGroups and NextHops.
func (a *AFTData) FilterByPrefixes(wantPrefixes map[string]bool) *AFTData {
	// Track which prefixes and NHG IDs we want to keep
	filteredPrefixes := make(map[string]uint64, len(wantPrefixes))
	usedNHGIDs := make(map[uint64]bool)

	// Copy only wanted prefixes and track which NHGs are used
	for prefix := range wantPrefixes {
		if nhgID, ok := a.Prefixes[prefix]; ok {
			filteredPrefixes[prefix] = nhgID
			usedNHGIDs[nhgID] = true
		}
	}

	// Filter NextHopGroups to only include those referenced by wantPrefixes
	filteredNHGs := make(map[uint64]*aftNextHopGroup, len(usedNHGIDs))
	usedNHIDs := make(map[uint64]bool)
	for nhgID := range usedNHGIDs {
		if nhg, ok := a.NextHopGroups[nhgID]; ok {
			filteredNHGs[nhgID] = nhg
			// Collect all NH IDs from this NHG
			for _, nhID := range nhg.NHIDs {
				usedNHIDs[nhID] = true
			}
		}
	}

	// Filter NextHops to only include those referenced by the filtered NextHopGroups
	filteredNHs := make(map[uint64]*aftNextHop, len(usedNHIDs))
	for nhID := range usedNHIDs {
		if nh, ok := a.NextHops[nhID]; ok {
			filteredNHs[nhID] = nh
		}
	}

	return &AFTData{
		Prefixes:      filteredPrefixes,
		NextHopGroups: filteredNHGs,
		NextHops:      filteredNHs,
	}
}

type prefixRecord struct {
	nhgID          uint64
	ts             int64
	hasNHG         bool
	hasStatePrefix bool
}

type nhgRecord struct {
	data *aftNextHopGroup
	ts   int64
}

type nhRecord struct {
	data *aftNextHop
	ts   int64
}

// aftCache stores compact typed AFT state parsed directly on notification
// arrival, avoiding retention of raw *gnmipb.Notification trees in memory.
type aftCache struct {
	mu            sync.RWMutex
	target        string
	defaultNI     string
	prefixesV4    map[string]prefixRecord
	prefixesV6    map[string]prefixRecord
	nextHopGroups map[uint64]nhgRecord
	nextHops      map[uint64]nhRecord

	leafCount              int64
	addCount               int64
	updateCount            int64
	delCount               int64
	shortPrefixAtomicCount int64
}

// aftNextHopGroup represents an AFT next hop group.
type aftNextHopGroup struct {
	// NHIDs contains the next hop IDs that are part of this next hop group.
	NHIDs []uint64
	// NHWeights contains the weights of the next hops in this next hop group.
	NHWeights map[uint64]uint64
	// Conditionals contains the conditionals that are part of this next hop group.
	Conditionals []*aftNextHopGroupConditional
}

// aftNextHopGroupConditional represents a condition for an AFT next hop group.
type aftNextHopGroupConditional struct {
	// DSCP contains the DSCP bits that are part of this conditional.
	DSCP []uint8
	// NHGID contains the next hop group ID that is part of this conditional.
	NHGID uint64
}

// aftNextHop represents an AFT next hop.
type aftNextHop struct {
	// IntfName contains the interface name of the next hop.
	IntfName string
	// IP contains the IP address of the next hop.
	IP string
	// LSPName contains the LSP name of the next hop.
	LSPName string
}

// ToAFT materializes an AFTData snapshot directly from the compact typed maps.
func (ss *AFTStreamSession) ToAFT(_ *testing.T, _ *ondatra.DUTDevice) (*AFTData, error) {
	c := ss.Cache
	c.mu.RLock()
	defer c.mu.RUnlock()

	prefixes := make(map[string]uint64, len(c.prefixesV4)+len(c.prefixesV6))
	for prefix, rec := range c.prefixesV4 {
		if !rec.hasNHG || !rec.hasStatePrefix {
			return nil, fmt.Errorf("missing required fields for IPv4 prefix %s (hasPrefix=%v, hasNHG=%v)", prefix, rec.hasStatePrefix, rec.hasNHG)
		}
		prefixes[prefix] = rec.nhgID
	}
	for prefix, rec := range c.prefixesV6 {
		if !rec.hasNHG || !rec.hasStatePrefix {
			return nil, fmt.Errorf("missing required fields for IPv6 prefix %s (hasPrefix=%v, hasNHG=%v)", prefix, rec.hasStatePrefix, rec.hasNHG)
		}
		prefixes[prefix] = rec.nhgID
	}

	nhgs := make(map[uint64]*aftNextHopGroup, len(c.nextHopGroups))
	for id, rec := range c.nextHopGroups {
		if rec.data == nil {
			continue
		}
		if len(rec.data.NHIDs) != len(rec.data.NHWeights) {
			return nil, fmt.Errorf("missing weights for NHG %d: got %d NHIDs and %d weights", id, len(rec.data.NHIDs), len(rec.data.NHWeights))
		}
		nhgs[id] = &aftNextHopGroup{
			NHIDs:        slices.Clone(rec.data.NHIDs),
			NHWeights:    maps.Clone(rec.data.NHWeights),
			Conditionals: slices.Clone(rec.data.Conditionals),
		}
	}

	nhs := make(map[uint64]*aftNextHop, len(c.nextHops))
	for id, rec := range c.nextHops {
		if rec.data == nil {
			continue
		}
		// Match prior parseNH ErrNotExist behavior: omit next hops with no IP,
		// LSP, or interface.
		if rec.data.IP == "" && rec.data.LSPName == "" && rec.data.IntfName == "" {
			continue
		}
		cp := *rec.data
		nhs[id] = &cp
	}

	return &AFTData{
		Prefixes:      prefixes,
		NextHopGroups: nhgs,
		NextHops:      nhs,
	}, nil
}

// logMetadata sends cache metadata to the test log.
func (c *aftCache) logMetadata(t *testing.T, start time.Time, prefix string) {
	c.mu.RLock()
	leafCount := c.leafCount
	addCount := c.addCount
	updateCount := c.updateCount
	delCount := c.delCount
	shortPrefixCount := c.shortPrefixAtomicCount
	c.mu.RUnlock()

	msg := fmt.Sprintf("%s After %v: %s:%d %s:%d %s:%d %s:%d ",
		prefix,
		time.Since(start).Truncate(time.Millisecond),
		metadata.LeafCount, leafCount,
		metadata.AddCount, addCount,
		metadata.UpdateCount, updateCount,
		metadata.DelCount, delCount,
	)
	t.Log(msg)
	if shortPrefixCount > 0 {
		t.Logf("%s WARNING: received %d atomic AFT update notification(s) without an entry-level prefix (gNMI spec Section 2.1.1 compliance warning)", prefix, shortPrefixCount)
	}
}

func (a *AFTData) isCNHG(nhgID uint64) (bool, error) {
	// Assume we've already checked the nhgID exists.
	if len(a.NextHopGroups[nhgID].NHIDs) > 0 && len(a.NextHopGroups[nhgID].Conditionals) > 0 {
		return false, fmt.Errorf("the NHG has both NHs and conditionals. not clear if CNHG or leaf NHG")
	}
	if len(a.NextHopGroups[nhgID].Conditionals) > 0 {
		return true, nil
	}
	// If the NHG has no NHs and no conditionals, treat it as a leaf NHG.
	return false, nil
}

// resolveNHGCBF returns the possible next hops for a specific next-hop group
// ID and DSCP value.
func (a *AFTData) resolveNHGCBF(prefix string, nhgID uint64, dscp uint8) ([]*aftNextHop, error) {
	visited := map[uint64]bool{} // Track NHGs we've seen in case of circular references.
	for {
		if _, ok := a.NextHopGroups[nhgID]; !ok {
			return nil, fmt.Errorf("missing reference for prefix %s, NHG %d not found: %w", prefix, nhgID, ErrNotExist)
		}
		isCNHG, err := a.isCNHG(nhgID)
		if err != nil {
			return nil, fmt.Errorf("error in prefix %s, error reading NHG %d: %v", prefix, nhgID, err)
		}
		if !isCNHG {
			// This is a leaf, non-conditional NHG node. Terminate.
			break
		}
		// We look up each ID in visited and add all IDs to visited. This should always terminate.
		if _, ok := visited[nhgID]; ok {
			return nil, fmt.Errorf("circular reference for prefix %s, NHG %d already seen", prefix, nhgID)
		}
		visited[nhgID] = true
		match := false
		for _, c := range a.NextHopGroups[nhgID].Conditionals {
			for _, d := range c.DSCP {
				if d == dscp {
					if match {
						// We already matched a different conditional. Undefined behavior.
						return nil, fmt.Errorf("undefined behavior for prefix %s, multiple conditionals apply", prefix)
					}
					match = true
					nhgID = c.NHGID
					break
				}
			}
		}
		if !match {
			return nil, nil // No conditionals matched. Return empty NH slice (nil).
		}
	}
	nhIDs := a.NextHopGroups[nhgID].NHIDs
	nhs := make([]*aftNextHop, 0, len(nhIDs))
	for _, nhID := range nhIDs {
		nh, ok := a.NextHops[nhID]
		if !ok {
			return nil, fmt.Errorf("missing reference for prefix %s, NH %d not found, %w", prefix, nhID, ErrNotExist)
		}
		nhs = append(nhs, nh)
	}
	return nhs, nil
}

// elemAt indexes into the virtual concatenation of a followed by b without
// allocating a combined slice.
func elemAt(a, b []*gnmipb.PathElem, idx int) *gnmipb.PathElem {
	if idx < len(a) {
		return a[idx]
	}
	return b[idx-len(a)]
}

// findAFTInPath scans the virtual concatenation (a + b) to find "afts" and an
// optional network-instance name, returning (aftIdx, niName) or (-1, "") if
// not found.
func findAFTInPath(a, b []*gnmipb.PathElem) (int, string) {
	total := len(a) + len(b)
	var niName string
	for i := 0; i < total; i++ {
		e := elemAt(a, b, i)
		switch e.GetName() {
		case "network-instance":
			if k := e.GetKey(); k != nil {
				niName = k["name"]
			}
		case "afts":
			return i, niName
		}
	}
	return -1, niName
}

func (c *aftCache) matchesNI(niName string) bool {
	return niName == "" || c.defaultNI == "" || niName == c.defaultNI
}

func (c *aftCache) hasMatchingAFTUpdate(prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update) bool {
	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		aftIdx, niName := findAFTInPath(prefixElems, uElems)
		if aftIdx < 0 || len(prefixElems)+len(uElems) < aftIdx+2 || !c.matchesNI(niName) {
			continue
		}
		switch elemAt(prefixElems, uElems, aftIdx+1).GetName() {
		case "ipv4-unicast", "ipv6-unicast", "next-hop-groups", "next-hops":
			return true
		}
	}
	return false
}

func (c *aftCache) addAFTNotification(n *gnmipb.SubscribeResponse) error {
	if n.GetSyncResponse() {
		return nil
	}

	update := n.GetUpdate()
	if update == nil {
		return fmt.Errorf("SubscribeResponse missing Update: %v", n)
	}

	ts := update.GetTimestamp()
	prefixElems := update.GetPrefix().GetElem()
	updates := update.GetUpdate()
	deletes := update.GetDelete()
	atomic := update.GetAtomic()
	if atomic && len(deletes) > 0 {
		return errors.New("atomic deletes unsupported")
	}

	if len(updates) > 0 {
		if atomic {
			return c.applyAtomicUpdates(prefixElems, updates, ts)
		}
		if c.hasMatchingAFTUpdate(prefixElems, updates) {
			return fmt.Errorf("%w: atomic AFT container requires atomic=true", errNonAtomicUpdate)
		}
	}
	if len(deletes) > 0 {
		c.applyDeletes(prefixElems, deletes, ts)
	}
	return nil
}

type aftEntryKey struct {
	table string
	key   string
}

func extractEntryKey(table string, entryElem *gnmipb.PathElem) string {
	switch table {
	case "ipv4-unicast", "ipv6-unicast":
		return entryElem.GetKey()["prefix"]
	case "next-hop-groups":
		return entryElem.GetKey()["id"]
	case "next-hops":
		return entryElem.GetKey()["index"]
	default:
		return ""
	}
}

func (c *aftCache) applyAtomicUpdates(prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update, ts int64) error {
	// Fast path: prefix already identifies the AFT table and entry key
	// (standard for atomic per-entry updates).
	if aftIdx, niName := findAFTInPath(prefixElems, nil); aftIdx >= 0 && len(prefixElems) >= aftIdx+3 {
		if !c.matchesNI(niName) {
			return nil
		}
		table := prefixElems[aftIdx+1].GetName()
		if key := extractEntryKey(table, prefixElems[aftIdx+2]); key != "" {
			return c.replaceEntry(table, key, prefixElems, updates, ts)
		}
	}

	// Short-prefix atomic path: group updates by (table, entryKey) so every
	// touched entry is replaced as a whole, and record a compliance warning.
	grouped := make(map[aftEntryKey][]*gnmipb.Update)
	var order []aftEntryKey
	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		aftIdx, niName := findAFTInPath(prefixElems, uElems)
		if aftIdx < 0 || len(prefixElems)+len(uElems) < aftIdx+3 || !c.matchesNI(niName) {
			continue
		}
		table := elemAt(prefixElems, uElems, aftIdx+1).GetName()
		key := extractEntryKey(table, elemAt(prefixElems, uElems, aftIdx+2))
		if key == "" {
			continue
		}
		ek := aftEntryKey{table: table, key: key}
		if _, exists := grouped[ek]; !exists {
			order = append(order, ek)
		}
		grouped[ek] = append(grouped[ek], u)
	}
	if len(order) == 0 {
		return nil
	}

	c.mu.Lock()
	c.shortPrefixAtomicCount++
	c.mu.Unlock()

	staleOnly := true
	for _, ek := range order {
		err := c.replaceEntry(ek.table, ek.key, prefixElems, grouped[ek], ts)
		if err != nil {
			if errors.Is(err, cache.ErrStale) {
				continue
			}
			return err
		}
		staleOnly = false
	}
	if staleOnly {
		return cache.ErrStale
	}
	return nil
}

func (c *aftCache) replaceEntry(table, key string, prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update, ts int64) error {
	switch table {
	case "ipv4-unicast":
		return c.replacePrefix(false, key, prefixElems, updates, ts)
	case "ipv6-unicast":
		return c.replacePrefix(true, key, prefixElems, updates, ts)
	case "next-hop-groups":
		return c.replaceNHG(key, prefixElems, updates, ts)
	case "next-hops":
		return c.replaceNH(key, prefixElems, updates, ts)
	default:
		return nil
	}
}

func (c *aftCache) replacePrefix(isV6 bool, prefix string, prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update, ts int64) error {
	var nhgID uint64
	var gotNHG bool
	var gotStatePrefix bool

	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		total := len(prefixElems) + len(uElems)
		if total < 2 {
			continue
		}
		if elemAt(prefixElems, uElems, total-2).GetName() != "state" {
			continue
		}
		last := elemAt(prefixElems, uElems, total-1).GetName()
		switch last {
		case "next-hop-group":
			nhgID = u.GetVal().GetUintVal()
			gotNHG = true
		case "prefix":
			if val := u.GetVal().GetStringVal(); val != prefix {
				return fmt.Errorf("prefix mismatch: key=%q state=%q", prefix, val)
			}
			gotStatePrefix = true
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.prefixesV4
	if isV6 {
		m = c.prefixesV6
	}
	if old, ok := m[prefix]; ok {
		if ts > 0 && ts < old.ts {
			return cache.ErrStale
		}
		if ts > 0 && ts == old.ts && old.hasNHG == gotNHG && old.nhgID == nhgID && old.hasStatePrefix == gotStatePrefix {
			return cache.ErrStale
		}
	} else {
		c.leafCount++
		c.addCount++
	}
	m[prefix] = prefixRecord{nhgID: nhgID, ts: ts, hasNHG: gotNHG, hasStatePrefix: gotStatePrefix}
	c.updateCount += int64(len(updates))
	return nil
}

func (c *aftCache) replaceNHG(idStr string, prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update, ts int64) error {
	nhgID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return err
	}

	// Detect conditional next-hop-groups (unsupported per prior parseNHG
	// behavior).
	isConditional := false
	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		total := len(prefixElems) + len(uElems)
		for i := 0; i < total; i++ {
			if elemAt(prefixElems, uElems, i).GetName() == "condition" {
				isConditional = true
				break
			}
		}
		if isConditional {
			break
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	oldRec, exists := c.nextHopGroups[nhgID]
	if exists && ts > 0 && ts < oldRec.ts {
		return cache.ErrStale
	}
	if isConditional {
		if exists {
			delete(c.nextHopGroups, nhgID)
			c.leafCount--
			c.delCount++
		}
		return nil
	}

	nhg := &aftNextHopGroup{
		NHWeights: make(map[uint64]uint64),
	}
	seen := make(map[uint64]struct{})

	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		total := len(prefixElems) + len(uElems)
		if total < 2 {
			continue
		}
		parent := elemAt(prefixElems, uElems, total-2).GetName()
		last := elemAt(prefixElems, uElems, total-1).GetName()
		if parent != "state" {
			continue
		}
		switch last {
		case "id":
			if gotID := u.GetVal().GetUintVal(); gotID != nhgID {
				return fmt.Errorf("NHG id mismatch between prefix (%d) and update (%d)", nhgID, gotID)
			}
		case "index":
			id := u.GetVal().GetUintVal()
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				nhg.NHIDs = append(nhg.NHIDs, id)
			}
		case "weight":
			// Locate enclosing next-hop[index=...] key element.
			var nhKeyStr string
			for i := total - 3; i >= 0; i-- {
				e := elemAt(prefixElems, uElems, i)
				if e.GetName() == "next-hop" {
					nhKeyStr = e.GetKey()["index"]
					break
				}
			}
			if nhKeyStr == "" {
				return fmt.Errorf("missing next-hop index key for weight in NHG %d", nhgID)
			}
			memberNHID, err := strconv.ParseUint(nhKeyStr, 10, 64)
			if err != nil {
				return err
			}
			nhg.NHWeights[memberNHID] = u.GetVal().GetUintVal()
		}
	}

	if !exists {
		c.leafCount++
		c.addCount++
	}
	c.nextHopGroups[nhgID] = nhgRecord{data: nhg, ts: ts}
	c.updateCount += int64(len(updates))
	return nil
}

func (c *aftCache) replaceNH(idxStr string, prefixElems []*gnmipb.PathElem, updates []*gnmipb.Update, ts int64) error {
	nhID, err := strconv.ParseUint(idxStr, 10, 64)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	oldRec, exists := c.nextHops[nhID]
	if exists && ts > 0 && ts < oldRec.ts {
		return cache.ErrStale
	}

	nh := &aftNextHop{}
	for _, u := range updates {
		uElems := u.GetPath().GetElem()
		total := len(prefixElems) + len(uElems)
		if total < 2 {
			continue
		}
		parent := elemAt(prefixElems, uElems, total-2).GetName()
		last := elemAt(prefixElems, uElems, total-1).GetName()
		if parent != "state" {
			continue
		}
		switch last {
		case "index":
			if gotID := u.GetVal().GetUintVal(); gotID != nhID {
				return fmt.Errorf("NH index mismatch between prefix (%d) and update (%d)", nhID, gotID)
			}
		case "ip-address":
			nh.IP = u.GetVal().GetStringVal()
		case "lsp-name":
			nh.LSPName = u.GetVal().GetStringVal()
		case "interface":
			if total >= 3 && elemAt(prefixElems, uElems, total-3).GetName() == "interface-ref" {
				nh.IntfName = u.GetVal().GetStringVal()
			}
		}
	}

	if !exists {
		c.leafCount++
		c.addCount++
	}
	c.nextHops[nhID] = nhRecord{data: nh, ts: ts}
	c.updateCount += int64(len(updates))
	return nil
}

func shouldDelete(existingTS, delTS int64) bool {
	return existingTS <= delTS
}

func isAFTAncestorDelete(prefixElems, dElems []*gnmipb.PathElem, defaultNI string) bool {
	total := len(prefixElems) + len(dElems)
	if total == 0 {
		return true
	}
	if elemAt(prefixElems, dElems, 0).GetName() != "network-instances" {
		return false
	}
	if total == 1 {
		return true
	}
	if total == 2 {
		e := elemAt(prefixElems, dElems, 1)
		if e.GetName() != "network-instance" {
			return false
		}
		niName := e.GetKey()["name"]
		return niName == "" || defaultNI == "" || niName == defaultNI
	}
	return false
}

func (c *aftCache) applyDeletes(prefixElems []*gnmipb.PathElem, deletes []*gnmipb.Path, ts int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range deletes {
		dElems := d.GetElem()
		aftIdx, niName := findAFTInPath(prefixElems, dElems)
		if aftIdx < 0 {
			if isAFTAncestorDelete(prefixElems, dElems, c.defaultNI) {
				c.updateCount++
				c.deleteAllMatchingLocked(ts)
			}
			continue
		}
		if !c.matchesNI(niName) {
			continue
		}
		total := len(prefixElems) + len(dElems)
		c.updateCount++

		// Subtree delete of entire afts container.
		if total == aftIdx+1 {
			c.deleteAllMatchingLocked(ts)
			continue
		}

		// Per gNMI spec Section 2.1.1, any delete at or below an atomic entry
		// key removes the entire entry; table-level deletes remove all matching
		// entries in that table.
		table := elemAt(prefixElems, dElems, aftIdx+1).GetName()
		switch table {
		case "ipv4-unicast":
			if total == aftIdx+2 || (total == aftIdx+3 && elemAt(prefixElems, dElems, aftIdx+2).GetKey()["prefix"] == "") {
				for prefix, rec := range c.prefixesV4 {
					if shouldDelete(rec.ts, ts) {
						delete(c.prefixesV4, prefix)
						c.leafCount--
						c.delCount++
					}
				}
				continue
			}
			if prefix := elemAt(prefixElems, dElems, aftIdx+2).GetKey()["prefix"]; prefix != "" {
				if rec, ok := c.prefixesV4[prefix]; ok && shouldDelete(rec.ts, ts) {
					delete(c.prefixesV4, prefix)
					c.leafCount--
					c.delCount++
				}
			}

		case "ipv6-unicast":
			if total == aftIdx+2 || (total == aftIdx+3 && elemAt(prefixElems, dElems, aftIdx+2).GetKey()["prefix"] == "") {
				for prefix, rec := range c.prefixesV6 {
					if shouldDelete(rec.ts, ts) {
						delete(c.prefixesV6, prefix)
						c.leafCount--
						c.delCount++
					}
				}
				continue
			}
			if prefix := elemAt(prefixElems, dElems, aftIdx+2).GetKey()["prefix"]; prefix != "" {
				if rec, ok := c.prefixesV6[prefix]; ok && shouldDelete(rec.ts, ts) {
					delete(c.prefixesV6, prefix)
					c.leafCount--
					c.delCount++
				}
			}

		case "next-hop-groups":
			if total == aftIdx+2 || (total == aftIdx+3 && elemAt(prefixElems, dElems, aftIdx+2).GetKey()["id"] == "") {
				for id, rec := range c.nextHopGroups {
					if shouldDelete(rec.ts, ts) {
						delete(c.nextHopGroups, id)
						c.leafCount--
						c.delCount++
					}
				}
				continue
			}
			idStr := elemAt(prefixElems, dElems, aftIdx+2).GetKey()["id"]
			nhgID, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil {
				continue
			}
			if rec, ok := c.nextHopGroups[nhgID]; ok && shouldDelete(rec.ts, ts) {
				delete(c.nextHopGroups, nhgID)
				c.leafCount--
				c.delCount++
			}

		case "next-hops":
			if total == aftIdx+2 || (total == aftIdx+3 && elemAt(prefixElems, dElems, aftIdx+2).GetKey()["index"] == "") {
				for id, rec := range c.nextHops {
					if shouldDelete(rec.ts, ts) {
						delete(c.nextHops, id)
						c.leafCount--
						c.delCount++
					}
				}
				continue
			}
			idxStr := elemAt(prefixElems, dElems, aftIdx+2).GetKey()["index"]
			nhID, err := strconv.ParseUint(idxStr, 10, 64)
			if err != nil {
				continue
			}
			if rec, ok := c.nextHops[nhID]; ok && shouldDelete(rec.ts, ts) {
				delete(c.nextHops, nhID)
				c.leafCount--
				c.delCount++
			}
		}
	}
}

func (c *aftCache) deleteAllMatchingLocked(ts int64) {
	for prefix, rec := range c.prefixesV4 {
		if shouldDelete(rec.ts, ts) {
			delete(c.prefixesV4, prefix)
			c.leafCount--
			c.delCount++
		}
	}
	for prefix, rec := range c.prefixesV6 {
		if shouldDelete(rec.ts, ts) {
			delete(c.prefixesV6, prefix)
			c.leafCount--
			c.delCount++
		}
	}
	for id, rec := range c.nextHopGroups {
		if shouldDelete(rec.ts, ts) {
			delete(c.nextHopGroups, id)
			c.leafCount--
			c.delCount++
		}
	}
	for id, rec := range c.nextHops {
		if shouldDelete(rec.ts, ts) {
			delete(c.nextHops, id)
			c.leafCount--
			c.delCount++
		}
	}
}

func newAFTCache(target, defaultNI string) *aftCache {
	return &aftCache{
		target:        target,
		defaultNI:     defaultNI,
		prefixesV4:    make(map[string]prefixRecord),
		prefixesV6:    make(map[string]prefixRecord),
		nextHopGroups: make(map[uint64]nhgRecord),
		nextHops:      make(map[uint64]nhRecord),
	}
}

type aftSubscriptionResponse struct {
	notification *gnmipb.SubscribeResponse
	err          error
}

// aftSubscribe subscribes to a gNMI client and creates a channel to read from the subscription
// stream asynchronously.
// TODO: Split out the watching logic into a blocking function.
// This is somewhat bad practice. I was surprised that this function spawned a goroutine.
// Functions should not return if they spawn goroutines. (Assume the caller will cancel the context
// on return.)
func aftSubscribe(ctx context.Context, t *testing.T, c gnmipb.GNMIClient, dut *ondatra.DUTDevice) <-chan *aftSubscriptionResponse {
	sub, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("error in Subscribe(): %v", err)
	}
	req, err := checkForRoutesRequest(dut)
	if err != nil {
		t.Fatalf("error preparing subscribe request: %v", err)
	}
	t.Logf("Sending subscribe request: %v", req)
	if err := sub.Send(req); err != nil {
		t.Fatalf("error sending subscribe request %v: %v", req, err)
	}

	buffer := make(chan *aftSubscriptionResponse, aftBufferSize)
	// Don't need to close the buffer channel. We don't need that signal, the stream stopping logic is with the consumer.
	go func() {
		for {
			n, err := sub.Recv()
			resp := &aftSubscriptionResponse{n, err}
			select {
			case buffer <- resp:
			case <-ctx.Done():
				// Context cancellation also makes sub.Recv() return an error. We rely on the out channel's
				// buffer filling up or random chance (select picks a random available case) to hit
				// <-ctx.Done() and return.
				return
			}
		}
	}()
	return buffer
}

// AFTStreamSession represents a single gNMI AFT streaming session and cached AFT state. It contains
// a subscription that can be used across multiple calls to ListenUntil().
type AFTStreamSession struct {
	buffer            <-chan *aftSubscriptionResponse
	Cache             *aftCache
	start             time.Time
	notifications     []*gnmipb.SubscribeResponse
	missingPrefixes   map[string]bool
	failingNHPrefixes map[string]bool
	debugMode         bool
}

func (ss *AFTStreamSession) sessionPrefix() string {
	return fmt.Sprintf("[%s-%d]", ss.Cache.target, ss.start.UnixNano())
}

// NewAFTStreamSession constructs an AFTStreamSession. It subscribes to a given gNMI client.
func NewAFTStreamSession(ctx context.Context, t *testing.T, c gnmipb.GNMIClient, dut *ondatra.DUTDevice) *AFTStreamSession {
	return &AFTStreamSession{
		buffer:            aftSubscribe(ctx, t, c, dut),
		Cache:             newAFTCache(dut.Name(), deviations.DefaultNetworkInstance(dut)),
		notifications:     []*gnmipb.SubscribeResponse{},
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
		debugMode:         false,
	}
}

// WithDebug enables the storage of all gNMI notifications for debugging purposes.
// Warning: This will significantly increase memory usage.
func (ss *AFTStreamSession) WithDebug() *AFTStreamSession {
	ss.debugMode = true
	return ss
}

// NotificationHook is a function that will be called when each notification is received, before updating the AFT cache.
type NotificationHook struct {
	// Description is a human readable description of the hook.
	Description string
	// NotificationFunc is the function that will be called when each notification is received.
	NotificationFunc func(c *aftCache, n *gnmipb.SubscribeResponse) error
}

// PeriodicHook is a function that will be called on a regular interval with the current AFT cache.
type PeriodicHook struct {
	Description  string
	PeriodicFunc func(ss *AFTStreamSession) (bool, error)
}

// loggingPeriodicHook prints AFT stats to the log on a regular interval during an AFT stream.
func loggingPeriodicHook(t *testing.T, start time.Time) PeriodicHook {
	return PeriodicHook{
		Description: "Log stream stats",
		PeriodicFunc: func(ss *AFTStreamSession) (bool, error) {
			ss.Cache.logMetadata(t, start, ss.sessionPrefix())
			return false, nil
		},
	}
}

func (ss *AFTStreamSession) loggingFinal(t *testing.T) {
	prefix := ss.sessionPrefix()
	ss.Cache.logMetadata(t, ss.start, prefix)
	t.Logf("%s After %v: Finished streaming.", prefix, time.Since(ss.start).Truncate(time.Millisecond))
	if len(ss.missingPrefixes) > 0 {
		filename, err := writeMissingPrefixes(t, ss.missingPrefixes, ss.Cache.target, ss.start)
		if err != nil {
			t.Errorf("%s error writing missing prefixes: %v", prefix, err)
		} else {
			t.Logf("%s Wrote missing prefixes to %s", prefix, filename)
		}
	}
	if len(ss.failingNHPrefixes) > 0 {
		filename, err := writeFailingNHPrefixes(t, ss.failingNHPrefixes, ss.Cache.target, ss.start)
		if err != nil {
			t.Errorf("%s error writing failing NH prefixes: %v", prefix, err)
		} else {
			t.Logf("%s Wrote failing NH prefixes to %s", prefix, filename)
		}
	}
	if len(ss.notifications) > 0 {
		filename, err := writeNotifications(t, ss.notifications, ss.Cache.target, ss.start)
		if err != nil {
			t.Errorf("%s error writing notifications: %v", prefix, err)
		} else {
			t.Logf("%s Wrote all received notifications to %s", prefix, filename)
		}
	}
	ss.notifications = nil
}

// ListenUntil updates AFT with notifications from a gNMI client in streaming mode, and stops
// listening based on the stoppingCondition hook.
func (ss *AFTStreamSession) ListenUntil(ctx context.Context, t *testing.T, timeout time.Duration, stoppingCondition PeriodicHook) {
	t.Helper()
	ss.ListenUntilPreUpdateHook(ctx, t, timeout, nil, stoppingCondition)
}

// ListenUntilPreUpdateHook updates AFT with notifications from a gNMI client in streaming mode,
// and stops listening based on the stoppingCondition hook. It also runs the preUpdateHooks before updating the AFT cache.
func (ss *AFTStreamSession) ListenUntilPreUpdateHook(ctx context.Context, t *testing.T, timeout time.Duration, preUpdateHooks []NotificationHook, stoppingCondition PeriodicHook) {
	t.Helper()
	ss.start = time.Now()
	defer ss.loggingFinal(t) // Print stats one more time before exiting even in case of fatal error.
	phs := []PeriodicHook{loggingPeriodicHook(t, ss.start), stoppingCondition}
	ss.listenUntil(ctx, t, timeout, preUpdateHooks, phs)
}

func (ss *AFTStreamSession) listenUntil(ctx context.Context, t *testing.T, timeout time.Duration, preUpdateHooks []NotificationHook, periodicHooks []PeriodicHook) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	periodicTicker := time.NewTicker(periodicInterval)
	defer periodicTicker.Stop()
	for {
		select {
		case resp := <-ss.buffer:
			if resp.err != nil {
				// Context cancellation can hit this code path from the stream sending a context cancellation error.
				t.Fatalf("error from gNMI stream: %v", resp.err)
			}
			// Only store notifications if debug mode is enabled.
			if ss.debugMode {
				ss.notifications = append(ss.notifications, resp.notification)
			}
			for _, hook := range preUpdateHooks {
				err := hook.NotificationFunc(ss.Cache, resp.notification)
				if err != nil {
					t.Fatalf("error in notificationHook %q: %v", hook.Description, err)
				}
			}
			err := ss.Cache.addAFTNotification(resp.notification)
			switch {
			case errors.Is(err, cache.ErrStale):
				continue
			case err != nil:
				t.Fatalf("error updating AFT cache with response %v: %v", resp.notification, err)
			}
		case <-periodicTicker.C:
			s := time.Now()
			for _, hook := range periodicHooks {
				done, err := hook.PeriodicFunc(ss)
				if err != nil {
					t.Fatalf("error in PeriodicHook %q: %v", hook.Description, err)
				}
				if done {
					return
				}
			}
			d := time.Since(s)
			if d > periodicDeadline {
				// If periodic hooks take too long, we can't guarantee we can catch up on processing
				// notifications and the test may slow down artificially. The periodic hooks should be
				// optimized or the parameters (periodicInterval, periodicDeadline) should be tuned.
				t.Fatalf("periodic hooks took %v, exceeding deadline of %v", d.Truncate(time.Millisecond), periodicDeadline)
			}
		case <-ctx.Done():
			t.Fatalf("context cancelled: %v", ctx.Err())
			return
		}
	}
}

// DeletionStoppingCondition returns a PeriodicHook which can be used to check if all given prefixes have been deleted.
func DeletionStoppingCondition(t *testing.T, dut *ondatra.DUTDevice, wantDeletePrefixes map[string]bool) PeriodicHook {
	return PeriodicHook{
		Description: "Route delete stopping condition",
		PeriodicFunc: func(ss *AFTStreamSession) (bool, error) {
			prefix := ss.sessionPrefix()
			a, err := ss.ToAFT(t, dut)
			if err != nil {
				return false, err
			}
			gotPrefixes := a.Prefixes
			nRem := 0
			for p := range gotPrefixes {
				if _, ok := wantDeletePrefixes[p]; ok {
					nRem++
				}
			}
			t.Logf("%s Got %d deleted prefixes out of %d wanted prefixes to delete so far.", prefix, len(wantDeletePrefixes)-nRem, len(wantDeletePrefixes))
			if nRem > 0 {
				return false, nil
			}
			t.Logf("%s Finished checking for deleted routes: %s", prefix, time.Now().String())
			return true, nil
		},
	}
}

// InitialSyncStoppingCondition returns a PeriodicHook which can be used to check if all wanted prefixes have been received with given next hop IP addresses.
func InitialSyncStoppingCondition(t *testing.T, dut *ondatra.DUTDevice, wantPrefixes, wantIPV4NHs, wantIPV6NHs map[string]bool) PeriodicHook {
	nhFailCount := 0
	const nhFailLimit = 20
	const maxSamplePrefixes = 10
	logDuration := func(start time.Time, stage, prefix string) {
		t.Logf("%s InitialSyncStoppingCondition: Stage: %s took %.2f seconds", prefix, stage, time.Since(start).Seconds())
	}
	return PeriodicHook{
		Description: "Initial sync stopping condition",
		PeriodicFunc: func(ss *AFTStreamSession) (bool, error) {
			prefix := ss.sessionPrefix()
			start := time.Now()
			a, err := ss.ToAFT(t, dut)
			logDuration(start, "Convert cache to AFT", prefix)
			if err != nil {
				return false, err
			}

			// Check prefixes.
			checkPrefixStart := time.Now()
			gotPrefixes := a.Prefixes
			nPrefixes := len(wantPrefixes)
			nGot := 0
			ss.missingPrefixes = make(map[string]bool)
			for p := range wantPrefixes {
				if _, ok := gotPrefixes[p]; ok {
					nGot++
				} else {
					ss.missingPrefixes[p] = true
				}
			}
			t.Logf("%s Got %d out of %d wanted prefixes so far.", prefix, nGot, nPrefixes)
			logDuration(checkPrefixStart, "Check Prefixes", prefix)
			if nGot < nPrefixes {
				t.Logf("%s %d missing prefixes", prefix, len(ss.missingPrefixes))
				// Log a sample of missing prefixes for easier debugging.
				i := 0
				for p := range ss.missingPrefixes {
					if i >= maxSamplePrefixes {
						break
					}
					t.Logf("%s Example missing prefix: %s", prefix, p)
					i++
				}
				return false, nil
			}
			ss.missingPrefixes = make(map[string]bool) // All prefixes are present, so clear the list.

			// Check next hops using O(H + N) memoization per unique
			// (nhgID, isIPv6).
			checkNHStart := time.Now()
			nCorrect := 0
			diffs := map[string]int{}
			ss.failingNHPrefixes = make(map[string]bool)
			type nhgEvalKey struct {
				nhgID  uint64
				isIPv6 bool
			}
			nhgDiffMemo := make(map[nhgEvalKey]string, len(a.NextHopGroups)*2)

			for p := range wantPrefixes {
				nhgID := gotPrefixes[p]
				key := nhgEvalKey{nhgID: nhgID, isIPv6: strings.Contains(p, ":")}
				diff, cached := nhgDiffMemo[key]
				if !cached {
					resolved, err := a.resolveNHGCBF(p, nhgID, 0)
					got := map[string]bool{}
					switch {
					// Skip the check if NH is not found, retry on next periodic hook. Report missing NHs after timeout.
					case errors.Is(err, ErrNotExist):
					case err != nil:
						return false, fmt.Errorf("error resolving next hops for prefix %v: %w", p, err)
					default:
						for _, r := range resolved {
							got[r.IP] = true
						}
					}
					want := wantIPV4NHs
					if key.isIPv6 {
						want = wantIPV6NHs
					}
					diff = cmp.Diff(want, got)
					nhgDiffMemo[key] = diff
				}
				if diff == "" {
					nCorrect++
					continue
				}
				ss.failingNHPrefixes[p] = true
				diffs[diff]++
			}
			for k, v := range diffs {
				t.Logf("%s %d mismatches of (-want +got):\n%s", prefix, v, k)
			}
			t.Logf("%s Got %d of %d correct NH so far.", prefix, nCorrect, nPrefixes)
			logDuration(checkNHStart, "Check Next Hops", prefix)
			if nCorrect != nPrefixes {
				nhFailCount++
				if nhFailCount == nhFailLimit {
					return false, fmt.Errorf("after %d tries, next hop validation still fails", nhFailLimit)
				}
				t.Logf("%s %d prefixes with failing next hop validation", prefix, len(ss.failingNHPrefixes))
				i := 0
				for p := range ss.failingNHPrefixes {
					if i >= 10 {
						break
					}
					t.Logf("%s Example prefix with failing next hop validation: %s", prefix, p)
					i++
				}
				return false, nil
			}
			ss.failingNHPrefixes = make(map[string]bool) // All NHs are correct, so clear the list.
			t.Logf("%s Initial sync stopping condition took %.2f sec", prefix, time.Since(start).Seconds())
			return true, nil
		},
	}
}

// AssertNextHopCount returns a PeriodicHook which can be used to check if all the given prefixes
// resolve to the expected number of next hops.
func AssertNextHopCount(t *testing.T, dut *ondatra.DUTDevice, wantPrefixes map[string]bool, wantNHCount int) PeriodicHook {
	logDuration := func(start time.Time, stage, prefix string) {
		t.Logf("%s AssertNextHopCount: Stage: %s took %.2f seconds", prefix, stage, time.Since(start).Seconds())
	}
	return PeriodicHook{
		Description: "Assert next hop count",
		PeriodicFunc: func(ss *AFTStreamSession) (bool, error) {
			prefix := ss.sessionPrefix()
			start := time.Now()
			a, err := ss.ToAFT(t, dut)
			logDuration(start, "Convert cache to AFT", prefix)
			if err != nil {
				return false, err
			}

			// Check prefixes.
			checkPrefixStart := time.Now()
			nPrefixes := len(wantPrefixes)
			nGot := 0
			for p := range wantPrefixes {
				if _, ok := a.Prefixes[p]; ok {
					nGot++
				}
			}
			t.Logf("%s Got %d out of %d wanted prefixes so far.", prefix, nGot, nPrefixes)
			logDuration(checkPrefixStart, "Check Prefixes", prefix)
			if nGot < nPrefixes {
				return false, nil
			}

			// Check next hops using O(H + N) memoization per unique NHG ID.
			checkNHStart := time.Now()
			nCorrect := 0
			defer func() {
				t.Logf("verified %d of %d prefixes in AssertNextHopCount.", nCorrect, len(wantPrefixes))
			}()
			defer logDuration(checkNHStart, "Check Next Hops", prefix)
			type nhCountResult struct {
				missing bool
				count   int
			}
			nhCountMemo := make(map[uint64]nhCountResult, len(a.NextHopGroups))
			for p := range wantPrefixes {
				nhgID := a.Prefixes[p]
				res, cached := nhCountMemo[nhgID]
				if !cached {
					resolved, err := a.resolveNHGCBF(p, nhgID, 0)
					switch {
					case errors.Is(err, ErrNotExist):
						res = nhCountResult{missing: true}
					case err != nil:
						return false, fmt.Errorf("error resolving next hops for prefix %v: %w", p, err)
					default:
						res = nhCountResult{count: len(resolved)}
					}
					nhCountMemo[nhgID] = res
				}
				if res.missing {
					continue
				}
				if res.count != wantNHCount {
					t.Logf("prefix %s has %d next hops, want %d", p, res.count, wantNHCount)
					return false, nil
				}
				nCorrect++
			}
			if nCorrect != nPrefixes {
				return false, nil
			}
			return true, nil
		},
	}
}

// VerifyAtomicFlagHook returns a NotificationHook which verifies that the atomic flag is set to true.
func VerifyAtomicFlagHook(t *testing.T) NotificationHook {
	t.Helper()
	return NotificationHook{
		Description: "Atomic update hook",
		NotificationFunc: func(c *aftCache, n *gnmipb.SubscribeResponse) error {
			if n.GetUpdate() == nil {
				return nil
			}
			atomicFlag := n.GetUpdate().GetAtomic()
			if n.GetUpdate().GetDelete() != nil {
				if atomicFlag {
					return fmt.Errorf("atomic flag is set to true for delete operation. Notification: %v", n.GetUpdate())
				}
				return nil
			}
			if !atomicFlag {
				return fmt.Errorf("atomic flag is not set to true. Notification: %v", n.GetUpdate())
			}
			return nil
		},
	}
}

func checkForRoutesRequest(dut *ondatra.DUTDevice) (*gnmipb.SubscribeRequest, error) {
	subReq := &gnmipb.SubscribeRequest_Subscribe{
		Subscribe: &gnmipb.SubscriptionList{
			Mode:     gnmipb.SubscriptionList_STREAM,
			Prefix:   &gnmipb.Path{Origin: "openconfig", Target: dut.Name()},
			Encoding: gnmipb.Encoding_PROTO,
		},
	}
	for _, paths := range subscriptionPaths(dut) {
		for _, p := range paths {
			pp, err := ygot.StringToPath(p, ygot.StructuredPath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse path: %v", err)
			}
			subReq.Subscribe.Subscription = append(subReq.Subscribe.Subscription, &gnmipb.Subscription{Path: pp, Mode: gnmipb.SubscriptionMode_ON_CHANGE})
		}
	}
	return &gnmipb.SubscribeRequest{Request: subReq}, nil
}

// getTestLogPath returns a path to a file in a directory suitable for test logs.
// If running under Bazel, it uses the undeclared outputs directory.
// Otherwise, it uses the test's temporary directory.
func getTestLogPath(t *testing.T, filename string) string {
	if outDir := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); outDir != "" {
		return filepath.Join(outDir, filename)
	}
	return filepath.Join(t.TempDir(), filename)
}

func writeMissingPrefixes(t *testing.T, missingPrefixes map[string]bool, target string, startTime time.Time) (string, error) {
	path := getTestLogPath(t, fmt.Sprintf("%s_%d_%s", target, startTime.UnixNano(), missingPrefixesFile))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for p := range missingPrefixes {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return "", err
		}
	}
	return path, nil
}

func writeFailingNHPrefixes(t *testing.T, failingNHPrefixes map[string]bool, target string, startTime time.Time) (string, error) {
	path := getTestLogPath(t, fmt.Sprintf("%s_%d_%s", target, startTime.UnixNano(), failingNHPrefixesFile))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for p := range failingNHPrefixes {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return "", err
		}
	}
	return path, nil
}

func writeNotifications(t *testing.T, notifications []*gnmipb.SubscribeResponse, target string, startTime time.Time) (string, error) {
	path := getTestLogPath(t, fmt.Sprintf("%s_%d_%s", target, startTime.UnixNano(), notificationsFile))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, n := range notifications {
		if _, err := fmt.Fprintln(f, n.String()); err != nil {
			return "", err
		}
	}
	return path, nil
}

// Notifications return the AFT stream notifications.
func (ss *AFTStreamSession) Notifications() []*gnmipb.SubscribeResponse {
	return ss.notifications
}

// notificationMatch tracks whether the expected update/delete notifications have been observed in the gNMI stream.
type notificationMatch struct {
	updateFound bool
	deleteFound bool
}

// NotificationExpectation defines prefixes that should appear as UPDATE and/or DELETE notifications in the gNMI stream.
type NotificationExpectation struct {
	AddPrefix        string
	DeletePrefix     string
	NotificationWait time.Duration
}

// WaitForNotification returns a PeriodicHook that waits for the expected update
// and/or delete notifications specified in NotificationExpectation. The hook
// completes successfully once all requested notifications are observed.
func WaitForNotification(t *testing.T, cfg NotificationExpectation) PeriodicHook {
	t.Helper()
	start := time.Now()
	return PeriodicHook{
		Description: "Wait for notification",
		PeriodicFunc: func(ss *AFTStreamSession) (bool, error) {
			match := notificationMatch{
				updateFound: cfg.AddPrefix == "",
				deleteFound: cfg.DeletePrefix == "",
			}
			for _, resp := range ss.Notifications() {
				update := resp.GetUpdate()
				if update == nil {
					// Ignore SubscribeResponse messages that do not contain an Update,
					// such as SyncResponse.
					continue
				}
				if !match.updateFound && hasUpdateNotification(update, cfg.AddPrefix) {
					match.updateFound = true
				}
				if !match.deleteFound && hasDeleteNotification(update, cfg.DeletePrefix) {
					match.deleteFound = true
				}
				if match.updateFound && match.deleteFound {
					return true, nil
				}
			}
			if time.Since(start) > cfg.NotificationWait {
				return false, fmt.Errorf("timed out waiting for update(%q) delete(%q)", cfg.AddPrefix, cfg.DeletePrefix)
			}
			return false, nil
		},
	}
}

// pathHasPrefixKey returns true if path contains the supplied route-prefix key.
func pathHasPrefixKey(path *gnmipb.Path, prefix string) bool {
	if path == nil {
		return false
	}
	for _, elem := range path.GetElem() {
		for _, keyVal := range elem.GetKey() {
			if keyVal == prefix {
				return true
			}
		}
	}
	return false
}

// hasUpdateNotification returns true if the specified prefix appears in the
// notification prefix or in any UPDATE path. For atomic containers in
// Subscribe and Get responses, gNMI section 3.5.2.5 permits a server to place
// the list key in Notification.Prefix and make the UPDATE path relative to
// that prefix. Checking both locations handles this encoding as well as
// notifications with the key directly in an UPDATE path.
func hasUpdateNotification(update *gnmipb.Notification, prefix string) bool {
	if len(update.GetUpdate()) == 0 {
		return false
	}
	if pathHasPrefixKey(update.GetPrefix(), prefix) {
		return true
	}
	for _, upd := range update.GetUpdate() {
		if pathHasPrefixKey(upd.GetPath(), prefix) {
			return true
		}
	}
	return false
}

// hasDeleteNotification returns true if the specified prefix appears in the
// notification prefix or in any DELETE path. For atomic containers in
// Subscribe and Get responses, gNMI section 3.5.2.5 permits a server to place
// the list key in Notification.Prefix and make the DELETE path relative to
// that prefix. Checking both locations handles this encoding as well as
// notifications with the key directly in a DELETE path.
func hasDeleteNotification(update *gnmipb.Notification, prefix string) bool {
	if len(update.GetDelete()) == 0 {
		return false
	}
	if pathHasPrefixKey(update.GetPrefix(), prefix) {
		return true
	}
	for _, del := range update.GetDelete() {
		if pathHasPrefixKey(del, prefix) {
			return true
		}
	}
	return false
}
