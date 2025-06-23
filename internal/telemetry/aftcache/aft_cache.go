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
	"github.com/openconfig/featureprofiles/internal/telemetry/schema"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/metadata"
	"github.com/openconfig/ygot/ygot"

	log "github.com/golang/glog"
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
	periodicInterval = 4 * time.Minute
	// periodicDeadline is the deadline for all periodic hooks in a run. Should be < periodicInterval.
	periodicDeadline = 2 * time.Minute
	// aftBufferSize is the capacity of the internal channel queueing notifications from DUT
	// before applying them to our internal cache. It should be large enough to prevent DUT from
	// timing out from a pending send longer than the timeout while our internal cache is
	// locked because a periodic hook is running, which could take up to periodicDeadline. The channel
	// is preallocated, using a 64bit pointer per slot. We expect somewhat increased memory usage on
	// heap from a higher buffer value just because the buffer may contain multiple updates for the
	// same leaf, while our internal cache would not.
	// We expect the buffer to be large enough to hold 2M IPv4 prefixes and 512K IPv6 prefixes.
	aftBufferSize = 3000000
	// missingPrefixesFile is the name of the file where missing prefixes are written.
	missingPrefixesFile = "missing_prefixes.txt"
)

var (
	// ErrNotExist is an error returned when expected AFT elements are not found and so AFT is inconsistent.
	ErrNotExist = errors.New("does not exist")
	// ErrUnsupported is an error returned when AFT elements are not supported.
	ErrUnsupported  = errors.New("unsupported")
	missingPrefixes = make(map[string]bool)
)

var unusedPaths = []string{
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/prefix",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol",
	"/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/prefix",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol",
	"/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance",
}

var subscriptionPaths = map[string][]string{
	"prefix": {
		"network-instances/network-instance[name=default]/afts/ipv4-unicast/ipv4-entry",
		"network-instances/network-instance[name=default]/afts/ipv6-unicast/ipv6-entry",
	},
	"nhg": {
		"network-instances/network-instance[name=default]/afts/next-hop-groups/next-hop-group",
	},
	"nh": {
		"network-instances/network-instance[name=default]/afts/next-hops/next-hop",
	},
}

// TODO: - Rework to remove these paths and only use subscriptionPaths.
var cacheTraversalPaths = map[string][]string{
	"prefix": {
		"network-instances/network-instance/default/afts/ipv4-unicast",
		"network-instances/network-instance/default/afts/ipv6-unicast",
	},
	"nhg": {
		"network-instances/network-instance/default/afts/next-hop-groups",
	},
	"nh": {
		"network-instances/network-instance/default/afts/next-hops",
	},
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

// ToAFT Creates AFT maps with cache information.
func (c *aftCache) ToAFT() (*AFTData, error) {
	a := newAFT()
	prefixFunc := func(n *gnmipb.Notification) error {
		p, nhg, err := parsePrefix(n)
		if err != nil {
			return err
		}
		a.Prefixes[p] = nhg
		return nil
	}
	nhgFunc := func(n *gnmipb.Notification) error {
		nhg, data, err := parseNHG(n)
		switch {
		case errors.Is(err, ErrNotExist) || errors.Is(err, ErrUnsupported):
			log.Warningf("error parsing NHG: %v", err)
		case err != nil:
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
			log.Warningf("error parsing NH: %v", err)
		case err != nil:
			return err
		default:
			a.NextHops[nh] = data
		}
		return nil
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
func (c *aftCache) logMetadata(t *testing.T, start time.Time) error {
	m := c.cache.Metadata()[c.target]
	msg := fmt.Sprintf("After %v: ", time.Since(start).Truncate(time.Millisecond))
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
	err := c.cache.GnmiUpdate(n.GetUpdate())
	// fmt.Println("Notification: ", n)
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
func aftSubscribe(ctx context.Context, t *testing.T, c gnmipb.GNMIClient, target string) <-chan *aftSubscriptionResponse {
	sub, err := c.Subscribe(ctx)
	if err != nil {
		t.Fatalf("error in Subscribe(): %v", err)
	}
	req, err := checkForRoutesRequest(target)
	if err != nil {
		t.Fatalf("error preparing subscribe request: %v", err)
	}
	t.Logf("Sending subscribe request: %v", req)
	if err := sub.Send(req); err != nil {
		t.Fatalf("error sending subscribe request %v: %v", req, err)
	}

	buffer := make(chan *aftSubscriptionResponse, aftBufferSize)
	// Don't need to close the buffer channel. We don't need that signal, the stream stopping logic is with the consumer.
	// TODO: Change logic to remove the need for return after spawning a go routine.
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
	buffer <-chan *aftSubscriptionResponse
	Cache  *aftCache
	start  time.Time
}

// NewAFTStreamSession constructs an AFTStreamSession. It subscribes to a given gNMI client.
func NewAFTStreamSession(ctx context.Context, t *testing.T, c gnmipb.GNMIClient, target string) *AFTStreamSession {
	return &AFTStreamSession{
		buffer: aftSubscribe(ctx, t, c, target),
		Cache:  newAFTCache(target),
	}
}

// notificationHook is a function that will be called when each notification is received, before updating the AFT cache.
type notificationHook struct {
	description      string
	notificationFunc func(c *aftCache, n *gnmipb.SubscribeResponse) error
}

// PeriodicHook is a function that will be called on a regular interval with the current AFT cache.
type PeriodicHook struct {
	Description  string
	PeriodicFunc func(c *aftCache) (bool, error)
}

// loggingPeriodicHook prints AFT stats to the log on a regular interval during an AFT stream.
func loggingPeriodicHook(t *testing.T, start time.Time) PeriodicHook {
	return PeriodicHook{
		Description: "Log stream stats",
		PeriodicFunc: func(c *aftCache) (bool, error) {
			c.logMetadata(t, start)
			return false, nil
		},
	}
}

func (ss *AFTStreamSession) loggingFinal(t *testing.T) {
	ss.Cache.logMetadata(t, ss.start)
	t.Logf("After %v: Finished streaming.", time.Since(ss.start).Truncate(time.Millisecond))
	if len(missingPrefixes) == 0 {
		return
	}
	filename, err := writeMissingPrefixes(missingPrefixes)
	if err != nil {
		t.Errorf("error writing missing prefixes: %v", err)
	} else {
		t.Logf("Wrote missing prefixes to %s", filename)
	}
}

// ListenUntil updates AFT with notifications from a gNMI client in streaming mode, and stops
// listening based on the stoppingCondition hook.
func (ss *AFTStreamSession) ListenUntil(ctx context.Context, t *testing.T, timeout time.Duration, stoppingCondition PeriodicHook) {
	t.Helper()
	ss.start = time.Now()
	defer ss.loggingFinal(t) // Print stats one more time before exiting even in case of fatal error.
	var pnhs []notificationHook
	phs := []PeriodicHook{loggingPeriodicHook(t, ss.start), stoppingCondition}
	ss.listenUntil(ctx, t, timeout, pnhs, phs)
}

func (ss *AFTStreamSession) listenUntil(ctx context.Context, t *testing.T, timeout time.Duration, preUpdateHooks []notificationHook, periodicHooks []PeriodicHook) {
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

			for _, hook := range preUpdateHooks {
				err := hook.notificationFunc(ss.Cache, resp.notification)
				if err != nil {
					t.Fatalf("error in notificationHook %q: %v", hook.description, err)
				}
			}
			err := ss.Cache.addAFTNotification(resp.notification)
			switch {
			case errors.Is(err, cache.ErrStale):
			case err != nil:
				t.Fatalf("error updating AFT cache with response %v: %v", resp.notification, err)
			}
		case <-periodicTicker.C:
			s := time.Now()
			for _, hook := range periodicHooks {
				done, err := hook.PeriodicFunc(ss.Cache)
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

// InitialSyncStoppingCondition returns a PeriodicHook which can be used to check if all wanted prefixes have been received with given next hop IP addresses.
func InitialSyncStoppingCondition(t *testing.T, wantPrefixes, wantIPV4NHDests, wantIPV6NHDests map[string]bool) PeriodicHook {
	nhFailCount := 0
	const nhFailLimit = 20
	logDuration := func(start time.Time) {
		t.Logf("Initial sync stopping condition took %.2f sec", time.Since(start).Seconds())
	}
	return PeriodicHook{
		Description: "Initial sync stopping condition",
		PeriodicFunc: func(c *aftCache) (bool, error) {
			start := time.Now()
			defer logDuration(start)
			a, err := c.ToAFT()
			if err != nil {
				return false, err
			}
			t.Logf("Convert cache to AFT took %.10f seconds", time.Since(start).Seconds())

			// Check prefixes.
			gotPrefixes := a.Prefixes
			nPrefixes := len(wantPrefixes)
			nGot := 0
			missingPrefixes := map[string]bool{}
			for p := range wantPrefixes {
				if _, ok := gotPrefixes[p]; ok {
					nGot++
				} else {
					missingPrefixes[p] = true
				}
			}
			t.Logf("Got %d out of %d wanted prefixes so far.", nGot, nPrefixes)
			if nGot < nPrefixes {
				t.Logf("%d missing prefixes.", len(missingPrefixes))
				return false, nil
			}

			// Check next hops.
			nCorrect := 0
			diffs := map[string]int{}
			for p := range wantPrefixes {
				resolved, err := a.resolveRoute(p)
				got := map[string]bool{}
				switch {
				case errors.Is(err, ErrNotExist):
					log.Warningf("error resolving next hops for prefix %v: %v", p, err)
				case err != nil:
					return false, fmt.Errorf("error resolving next hops for prefix %v: %w", p, err)
				default:
					for _, r := range resolved {
						got[r.IP] = true
					}
				}
				want := wantIPV4NHDests
				if strings.Contains(p, ":") {
					want = wantIPV6NHDests
				}
				diff := cmp.Diff(want, got)
				if diff == "" {
					nCorrect++
					continue
				}
				if _, ok := diffs[diff]; !ok {
					diffs[diff] = 0
				}
				diffs[diff]++
			}
			for k, v := range diffs {
				t.Logf("%d mismatches of (-want +got):\n%s", v, k)
			}
			t.Logf("Got %d of %d correct NH so far.", nCorrect, nPrefixes)
			if nCorrect != nPrefixes {
				nhFailCount++
				if nhFailCount == nhFailLimit {
					return false, fmt.Errorf("after %d tries, next hop validation still fails", nhFailLimit)
				}
				return false, nil
			}
			return true, nil
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
func parseNHG(n *gnmipb.Notification) (uint64, *aftNextHopGroup, error) {
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
		log.Warningf("no next hop values were found in notification %v, %w", n, ErrNotExist)
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
func parsePrefix(n *gnmipb.Notification) (string, uint64, error) {
	// Normalizes paths for the "updates" in the gNMI notification.
	updates := schema.NotificationToPoints(n)
	if len(updates) == 0 {
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
	var nhgID uint64
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
			log.Warningf("unexpected path %q in prefix notification %v", path, n)
		}
	}
	if len(wantFields) < 2 {
		return "", 0, fmt.Errorf("missing required fields %v from the response %v", wantFields, n)
	}
	return prefix, nhgID, nil
}

func checkForRoutesRequest(target string) (*gnmipb.SubscribeRequest, error) {
	subReq := &gnmipb.SubscribeRequest_Subscribe{
		Subscribe: &gnmipb.SubscriptionList{
			Mode:     gnmipb.SubscriptionList_STREAM,
			Prefix:   &gnmipb.Path{Origin: "openconfig", Target: target},
			Encoding: gnmipb.Encoding_PROTO,
		},
	}
	for _, paths := range subscriptionPaths {
		for _, p := range paths {
			pp, err := ygot.StringToPath(p, ygot.StructuredPath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse path: %v", err)
			}
			subReq.Subscribe.Subscription = append(subReq.Subscribe.Subscription, &gnmipb.Subscription{Path: pp})
		}
	}
	return &gnmipb.SubscribeRequest{Request: subReq}, nil
}

func writeMissingPrefixes(missingPrefixes map[string]bool) (string, error) {
	absFilename, err := filepath.Abs(missingPrefixesFile)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(absFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for p := range missingPrefixes {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return "", err
		}
	}
	return absFilename, nil
}
