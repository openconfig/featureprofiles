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

// Package aftcache is a test library for storing a stream of AFT telemetry at full RIB scale
// in a local cache so we can periodically check if required test conditions are met,
// such as verifying that all expected prefixes are present.
package aftcache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/telemetry/schema"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/metadata"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

const (
	prefixPathV4              = "/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix"
	prefixNHGPathV4           = "/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group"
	prefixPathV6              = "/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix"
	prefixNHGPathV6           = "/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group"
	nextHopWeightPath         = "/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight"
	nextHopGroupConditionPath = "/network-instances/network-instance/afts/next-hop-groups/next-hop-group/condition"
	// periodicInterval is the time between execution of periodic hooks.
	periodicInterval = 2 * time.Minute
	// periodicDeadline is the deadline for all periodic hooks in a run. Should be < periodicInterval.
	periodicDeadline = 1*time.Minute + 45*time.Second
	// aftBufferSize is the capacity of the internal channel queueing notifications from DUT
	// before applying them to our internal cache. It should be large enough to prevent DUT from
	// timing out from a pending send longer than the timeout while our internal cache is
	// locked because a periodic hook is running, which could take up to periodicDeadline. The channel
	// is preallocated, using a 64bit pointer per slot. We expect somewhat increased memory usage on
	// heap from a higher buffer value just because the buffer may contain multiple updates for the
	// same leaf, while our internal cache would not.
	// We expect the buffer to be large enough to hold 2M IPv4 prefixes and 1M IPv6 prefixes.
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
)

var unusedPaths = []string{
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/prefix",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/octets-forwarded",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/counters/packets-forwarded",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/decapsulate-header",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/entry-metadata",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-network-instance",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/prefix",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/octets-forwarded",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/counters/packets-forwarded",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/decapsulate-header",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/entry-metadata",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-network-instance",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol",
	"/network-instances/network-instance/afts/next-hop-groups/next-hop-group/id",
	"/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/index",
	"/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group",
	"/network-instances/network-instance/afts/next-hops/next-hop/index",
	"/network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/subinterface",
	"/network-instances/network-instance/afts/next-hops/next-hop/state/counters/octets-forwarded",
	"/network-instances/network-instance/afts/next-hops/next-hop/state/counters/packets-forwarded",
	"/network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header",
	"/network-instances/network-instance/afts/next-hops/next-hop/state/mac-address",
	"/network-instances/network-instance/afts/next-hops/next-hop/state/origin-protocol",
}

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

// aftCache is the AFT streaming cache.
type aftCache struct {
	cache  *cache.Cache // Cache used to store AFT notifications during streaming.
	target string
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

// generateCacheTraversalPaths converts a map of subscription paths to a map of cache traversal paths.
// For example:
// input:
//
//	map[string][]string{
//		"prefix": []string{
//			"network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry",
//		},
//	}
//
// output:
//
//	map[string][]string{
//		"prefix": []string{
//			"openconfig/network-instances/network-instance/name/DEFAULT/afts/ipv4-unicast/ipv4-entry",
//		},
//	}
func generateCacheTraversalPaths(subscriptionPaths map[string][]string) (map[string][]string, error) {
	cachePaths := make(map[string][]string)
	for key, paths := range subscriptionPaths {
		var currentPaths []string
		for _, p := range paths {
			sp, err := ygot.StringToStructuredPath(p)
			if err != nil {
				return nil, fmt.Errorf("error parsing path %s: %w", p, err)
			}
			outParts := []string{"openconfig"}
			for _, elem := range sp.Elem {
				outParts = append(outParts, elem.Name)
				for _, keyVal := range elem.Key {
					outParts = append(outParts, keyVal)
				}
			}
			currentPaths = append(currentPaths, strings.Join(outParts, "/"))
		}
		cachePaths[key] = currentPaths
	}
	return cachePaths, nil
}

// ToAFT Creates AFT maps with cache information.
func (ss *AFTStreamSession) ToAFT(t *testing.T, dut *ondatra.DUTDevice) (*AFTData, error) {
	sessionPrefix := ss.sessionPrefix()
	c := ss.Cache
	a := newAFT()
	prefixFunc := func(n *gnmipb.Notification) error {
		p, nhg, err := parsePrefix(t, n, sessionPrefix)
		if err != nil {
			t.Logf("%s error in parsing prefix: %v", sessionPrefix, err)
			return err
		}
		a.Prefixes[p] = nhg
		return nil
	}
	nhgFunc := func(n *gnmipb.Notification) error {
		nhg, data, err := parseNHG(t, n)
		switch {
		case errors.Is(err, ErrNotExist) || errors.Is(err, ErrUnsupported):
			t.Logf("%s error parsing NHG: %v", sessionPrefix, err)
		case err != nil:
			t.Logf("%s error in parsing NHG: %v", sessionPrefix, err)
			return err
		default:
			a.NextHopGroups[nhg] = data
		}
		return nil
	}
	nhFunc := func(n *gnmipb.Notification) error {
		nh, data, err := parseNH(n)
		switch {
		case errors.Is(err, ErrNotExist):
			t.Logf("%s error parsing NH: %v", sessionPrefix, err)
		case err != nil:
			return err
		default:
			a.NextHops[nh] = data
		}
		return nil
	}
	cacheTraversalPaths, err := generateCacheTraversalPaths(subscriptionPaths(dut))
	if err != nil {
		return nil, err
	}
	parsers := []struct {
		paths []string
		f     func(n *gnmipb.Notification) error
	}{
		{
			paths: cacheTraversalPaths["prefix"],
			f:     prefixFunc,
		},
		{
			paths: cacheTraversalPaths["nhg"],
			f:     nhgFunc,
		},
		{
			paths: cacheTraversalPaths["nh"],
			f:     nhFunc,
		},
	}
	for _, p := range parsers {
		for _, path := range p.paths {
			if err := c.traverse(path, p.f); err != nil {
				return nil, err
			}
		}
	}
	return a, nil
}

// logMetadata sends cache metadata to testing log.
func (c *aftCache) logMetadata(t *testing.T, start time.Time, prefix string) error {
	m := c.cache.Metadata()[c.target]
	msg := fmt.Sprintf("%s After %v: ", prefix, time.Since(start).Truncate(time.Millisecond))
	fields := []string{metadata.LeafCount, metadata.AddCount, metadata.UpdateCount, metadata.DelCount}
	for _, f := range fields {
		v, err := m.GetInt(f)
		if err != nil {
			return err
		}
		msg += fmt.Sprintf("%s:%d ", f, v)
	}
	t.Log(msg)
	return nil
}

// traverse walks over portion of cache described by the provided
// openconfig path ocPath, and calls function f on the value at each leaf.
// ocPath is a unix path style string which can contain globs (*).
func (c *aftCache) traverse(ocPath string, f func(val *gnmipb.Notification) error) error {
	return c.cache.Query(c.target, strings.Split(ocPath, "/"), func(_ []string, _ *ctree.Leaf, val any) error {
		n, ok := val.(*gnmipb.Notification)
		if !ok {
			return fmt.Errorf("value is not a notification: %v", val)
		}
		return f(n)
	})
}

// ResolveRoute gets the possible next hops for a specific route.
// Uses 0 as the DSCP bits as a default. Usually this corresponds to a low priority traffic class.
// ResolveRoute should only be used in test cases where we don't expect to encounter CNHGs.
// For testing CNHGs, it's better to explicitly set DSCP with ResolveRouteCBF.
func (a *AFTData) resolveRoute(prefix string) ([]*aftNextHop, error) {
	return a.resolveRouteCBF(prefix, 0)
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

// ResolveRouteCBF gets the possible next hops for a specific route.
// dscp is the DSCP bits.
func (a *AFTData) resolveRouteCBF(prefix string, dscp uint8) ([]*aftNextHop, error) {
	if _, ok := a.Prefixes[prefix]; !ok {
		return nil, fmt.Errorf("missing prefix. want %s, %w", prefix, ErrNotExist)
	}
	nhgID := a.Prefixes[prefix]
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
	var nhs []*aftNextHop
	for _, nhID := range a.NextHopGroups[nhgID].NHIDs {
		if _, ok := a.NextHops[nhID]; !ok {
			return nil, fmt.Errorf("missing reference for prefix %s, NH %d not found, %w", prefix, nhID, ErrNotExist)
		}
		nhs = append(nhs, a.NextHops[nhID])
	}
	return nhs, nil
}

func (c *aftCache) addAFTNotification(n *gnmipb.SubscribeResponse) error {
	if n.GetSyncResponse() {
		// No-op for now.
		return nil
	}

	update := n.GetUpdate()
	if update == nil {
		return fmt.Errorf("SubscribeResponse missing Update: %v", n)
	}
	if update.GetPrefix() == nil {
		update.Prefix = &gnmipb.Path{}
	}
	prefix := update.GetPrefix()
	if prefix.GetOrigin() == "" {
		prefix.Origin = "openconfig"
	}
	if prefix.GetTarget() == "" {
		prefix.Target = c.target
	}
	err := c.cache.GnmiUpdate(update)

	if err != nil {
		return err
	}
	return nil
}

func newAFTCache(target string) *aftCache {
	return &aftCache{
		cache:  cache.New([]string{target}),
		target: target,
	}
}

func newAFT() *AFTData {
	return &AFTData{
		Prefixes:      map[string]uint64{},
		NextHopGroups: map[uint64]*aftNextHopGroup{},
		NextHops:      map[uint64]*aftNextHop{},
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
}

func (ss *AFTStreamSession) sessionPrefix() string {
	return fmt.Sprintf("[%s-%d]", ss.Cache.target, ss.start.UnixNano())
}

// NewAFTStreamSession constructs an AFTStreamSession. It subscribes to a given gNMI client.
func NewAFTStreamSession(ctx context.Context, t *testing.T, c gnmipb.GNMIClient, dut *ondatra.DUTDevice) *AFTStreamSession {
	return &AFTStreamSession{
		buffer:            aftSubscribe(ctx, t, c, dut),
		Cache:             newAFTCache(dut.Name()),
		notifications:     []*gnmipb.SubscribeResponse{},
		missingPrefixes:   make(map[string]bool),
		failingNHPrefixes: make(map[string]bool),
	}
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
	for {
		select {
		case resp := <-ss.buffer:
			if resp.err != nil {
				// Context cancellation can hit this code path from the stream sending a context cancellation error.
				t.Fatalf("error from gNMI stream: %v", resp.err)
			}
			ss.notifications = append(ss.notifications, resp.notification)

			for _, hook := range preUpdateHooks {
				err := hook.NotificationFunc(ss.Cache, resp.notification)
				if err != nil {
					t.Fatalf("error in notificationHook %q: %v", hook.Description, err)
				}
			}
			err := ss.Cache.addAFTNotification(resp.notification)
			switch {
			case errors.Is(err, cache.ErrStale):
				t.Logf("Received stale notification with timestamp %v (current time: %v)", time.Unix(0, resp.notification.GetUpdate().GetTimestamp()), time.Now())
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

			// Check next hops.
			checkNHStart := time.Now()
			nCorrect := 0
			diffs := map[string]int{}
			ss.failingNHPrefixes = make(map[string]bool)
			for p := range wantPrefixes {
				resolved, err := a.resolveRoute(p)
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
				if strings.Contains(p, ":") {
					want = wantIPV6NHs
				}
				diff := cmp.Diff(want, got)
				if diff == "" {
					nCorrect++
					continue
				}
				ss.failingNHPrefixes[p] = true
				if _, ok := diffs[diff]; !ok {
					diffs[diff] = 0
				}
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

			// Check next hops.
			checkNHStart := time.Now()
			nCorrect := 0
			defer t.Logf("verified %d of %d prefixes in AssertNextHopCount.", nCorrect, len(wantPrefixes))
			defer logDuration(checkNHStart, "Check Next Hops", prefix)
			for p := range wantPrefixes {
				resolved, err := a.resolveRoute(p)
				switch {
				// Skip the check if NH is not found, retry on next periodic hook. Report missing NHs after timeout.
				case errors.Is(err, ErrNotExist):
				case err != nil:
					return false, fmt.Errorf("error resolving next hops for prefix %v: %w", p, err)
				default:
					if len(resolved) != wantNHCount {
						t.Logf("prefix %s has %d next hops, want %d", p, len(resolved), wantNHCount)
						return false, nil
					}
					nCorrect++
				}
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

// parseNH parses AFT NH notification and returns NH, IP, and LSP information.
func parseNH(n *gnmipb.Notification) (uint64, *aftNextHop, error) {
	e := n.GetPrefix().GetElem()
	if len(e) < 5 {
		return 0, nil, fmt.Errorf("not enough elements in prefix.  Notification: %v", n)
	}
	val, ok := e[4].GetKey()["index"]
	if !ok {
		return 0, nil, fmt.Errorf("\"index\" not a key in element.  Notification: %v", n)
	}
	nhID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, nil, err
	}
	entries := map[uint64]bool{nhID: true}
	var path string
	// Loop over updates, looking for all instances of next hop.
	updates := schema.NotificationToPoints(n)
	for _, u := range updates {
		path, err = ygot.PathToSchemaPath(u.Path)
		switch {
		case err != nil:
			return 0, nil, err
		case strings.HasSuffix(path, "state/index"):
			entries[u.Val.GetUintVal()] = true
		}
	}
	// Ensure all next-hop entries are consistent.
	if len(entries) != 1 {
		return 0, nil, fmt.Errorf("the NH values do not match between Prefix and Update parts of message.  Notification: %v", n)
	}
	// Loop over update, looking for ip-address, lsp-name, and/or interface name.
	found := false
	nh := &aftNextHop{}
	for _, u := range updates {
		path, err = ygot.PathToSchemaPath(u.Path)
		switch {
		case err != nil:
			return 0, nil, err
		case strings.HasSuffix(path, "state/ip-address"):
			nh.IP = u.Val.GetStringVal()
			found = true
		case strings.HasSuffix(path, "state/lsp-name"):
			nh.LSPName = u.Val.GetStringVal()
			found = true
		case strings.HasSuffix(path, "interface-ref/state/interface"):
			nh.IntfName = u.Val.GetStringVal()
			found = true
		}
	}
	if !found {
		err = fmt.Errorf("ip-address, interface, nor lsp-name were found in notification %v. %w", n, ErrNotExist)
	}
	return nhID, nh, err
}

// parseNHG parses AFT NHG notification and return NHG and next hops from the notification.
func parseNHG(t *testing.T, n *gnmipb.Notification) (uint64, *aftNextHopGroup, error) {
	e := n.GetPrefix().GetElem()
	if len(e) < 5 {
		return 0, nil, fmt.Errorf("not enough elements in prefix.  Notification: %v", n)
	}
	val, ok := e[4].GetKey()["id"]
	if !ok {
		return 0, nil, fmt.Errorf("\"id\" not a key in element.  Notification: %v", n)
	}
	nhgID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, nil, err
	}
	var p string
	var updates []schema.Point
	entries := map[uint64]bool{nhgID: true}

	// Loop over updates, looking for instances of next hop groups and next hops.
	updates = schema.NotificationToPoints(n)

	nhg := &aftNextHopGroup{
		NHIDs:     []uint64{},
		NHWeights: map[uint64]uint64{},
	}
	for _, u := range updates {
		p, err = ygot.PathToSchemaPath(u.Path)
		if strings.HasPrefix(p, nextHopGroupConditionPath) {
			// Ignore conditional next hops groups for now.
			return 0, nil, fmt.Errorf("next hop group notification is conditional:%v, %w", n, ErrUnsupported)
		}
		switch {
		case err != nil:
			return 0, nil, err
		// Match for the path of the form:
		// /network-instances/network-instance/DEFAULT/afts/next-hop-groups/next-hop-group[id=<id>]/state/id
		case strings.HasSuffix(p, "state/id"):
			entries[u.Val.GetUintVal()] = true
		// Match for the path of the form:
		// /network-instances/network-instance/DEFAULT/afts/next-hop-groups/next-hop-group[id=<id>]/state/index
		case strings.HasSuffix(p, "state/index"):
			nhg.NHIDs = append(nhg.NHIDs, u.Val.GetUintVal())
		case p == nextHopWeightPath:
			nhID, err := strconv.ParseUint(u.Path.GetElem()[6].GetKey()["index"], 10, 64)
			if err != nil {
				return 0, nil, err
			}
			nhg.NHWeights[nhID] = u.Val.GetUintVal()
		}
	}
	if len(nhg.NHIDs) == 0 {
		t.Logf("no next hop values were found in notification %v, %v", n, ErrNotExist)
	}
	if len(entries) != 1 {
		err = fmt.Errorf("the NHG values do not match between Prefix and Update parts of message. Notification: %v, %w", n, err)
	}
	if len(nhg.NHIDs) != len(nhg.NHWeights) {
		err = fmt.Errorf("missing Weights for a few NHIDs. Notification: %v, %w", n, err)
	}
	return nhgID, nhg, err
}

// parsePrefix extracts the IP prefix and next-hop-group ID from an AFT prefix GNMI notification.
func parsePrefix(t *testing.T, n *gnmipb.Notification, sessionPrefix string) (string, uint64, error) {
	// Normalizes paths for the "updates" in the gNMI notification.
	updates := schema.NotificationToPoints(n)
	if len(updates) == 0 {
		t.Logf("no updates found in parsePrefix")
		return "", 0, fmt.Errorf("missing updates")
	}
	e := updates[0].Path.GetElem()
	if len(e) < 5 {
		return "", 0, fmt.Errorf("invalid prefix path in Notification: %v", n)
	}
	prefix, ok := updates[0].Path.GetElem()[4].GetKey()["prefix"]
	if !ok {
		return "", 0, fmt.Errorf("invalid prefix path")
	}
	wantFields := map[string]bool{}
	nhgID := uint64(0)
	for _, u := range updates {
		path, err := ygot.PathToSchemaPath(u.Path)
		if err != nil {
			return "", 0, fmt.Errorf("error converting path to schema path: %v", err)
		}
		switch {
		case path == prefixNHGPathV4 || path == prefixNHGPathV6:
			wantFields[path] = true
			nhgID = u.Val.GetUintVal()
		case path == prefixPathV4 || path == prefixPathV6:
			wantFields[path] = true
			if u.Val.GetStringVal() != prefix {
				return "", 0, fmt.Errorf("prefix mismatch")
			}
		// known unused paths
		case slices.Contains(unusedPaths, path):
		default:
			t.Logf("%s unexpected path %q in prefix notification %v", sessionPrefix, path, n)
		}
	}
	if len(wantFields) < 2 {
		return "", 0, fmt.Errorf("missing required fields %v from the response %v", wantFields, n)
	}
	return prefix, nhgID, nil
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
