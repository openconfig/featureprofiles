// Copyright 2023 Google LLC
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"

	"github.com/golang/glog"
)

type badgeState struct {
	Path   string
	Status string
}

// The report comment used to be written only from the GitHub webhook path,
// which runs when a pull request is pushed to or when someone comments
// "/fptest".  Test results, however, arrive later and separately, on the
// pubsub topic handled below, and that path only repainted the badge image.
//
// The effect was that the text in the report froze at whatever the tests
// were doing when they were launched -- usually "setup" -- and never showed
// the outcome.  A result only ever reached the text by accident, when
// somebody happened to run "/fptest" again after a run had finished, which
// in practice people only do after a failure.
//
// The code below closes that gap: when a test reaches a final state, the
// report is rebuilt from the badge metadata that populateObjectMetadata
// already knows how to read.
//
// Everything here is best effort.  Refreshing the report must never
// interfere with updating badges, which is the pre-existing job of this
// file and is relied on by the badge images embedded in every report.

// terminalTestStatus is the set of test states that cannot change again
// without a new test execution.  Intermediate states are deliberately
// excluded: they are already visible in the badge image, and refreshing on
// each of them would mean a repository clone and a comment edit for every
// state transition of every test.
var terminalTestStatus = map[string]bool{
	"success": true,
	"failure": true,
}

const (
	// reportCoalesceDelay is how long to wait for sibling results before
	// rebuilding the report.  Every test on every device publishes its own
	// badge message and they tend to land together, so a pull request with
	// one test across six virtual devices produces six messages within a
	// few seconds.  Pausing lets one rebuild cover all of them.
	reportCoalesceDelay = 30 * time.Second

	// reportTimeout bounds a single rebuild, which clones the pull request
	// and calls the GitHub API.  It matches the limit processEvent applies
	// to the webhook path.
	reportTimeout = 5 * time.Minute
)

// reportRefresh serialises report rebuilds per pull request commit.
//
// Two goroutines rebuilding the same report would race: each reads the badge
// metadata and then edits the same comment, so the slower reader can land
// last and overwrite newer results with older ones.  Rather than lock around
// the whole rebuild, a commit with a rebuild already pending records the new
// result in dirty and returns; the pending rebuild re-runs and picks it up.
// That collapses a burst into one rebuild while still guaranteeing the last
// result is never dropped.
var reportRefresh = struct {
	mu sync.Mutex
	// inFlight marks commits with a rebuild queued or running.
	inFlight map[string]bool
	// dirty marks commits that saw a new result while rebuilding.
	dirty map[string]bool
}{
	inFlight: map[string]bool{},
	dirty:    map[string]bool{},
}

// updateBadgeStatus updates the badgeState.Path in the gcpBucket
// to the new status value.  It requires the object to already exist
// and maintains the previous metadata values.
func updateBadgeStatus(ctx context.Context, bs *badgeState) error {
	storClient, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	objAttrs, err := storClient.Bucket(gcpBucket).Object(bs.Path).Attrs(ctx)
	if err != nil {
		return err
	}

	label, ok := objAttrs.Metadata["label"]
	if !ok {
		return fmt.Errorf("object %s missing metadata label", bs.Path)
	}

	buf, err := svgBadge(label, bs.Status)
	if err != nil {
		return err
	}

	obj := storClient.Bucket(gcpBucket).Object(bs.Path).NewWriter(ctx)
	obj.ContentType = objAttrs.ContentType
	obj.CacheControl = objAttrs.CacheControl
	obj.Metadata = objAttrs.Metadata
	obj.Metadata["status"] = bs.Status
	if _, err := buf.WriteTo(obj); err != nil {
		return err
	}

	return obj.Close()
}

// parseBadgePath returns the pull request number and head commit SHA encoded
// in a badge object path of the form:
//
//	badges/<pull request>/<head SHA>/<base64 test path>.<device>.svg
//
// The path is built by populateTestDetail.  The test path is base64 raw URL
// encoded, an alphabet that never contains "/", so the path always splits
// into exactly four parts.
func parseBadgePath(path string) (int, string, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != gcpBucketPrefix {
		return 0, "", fmt.Errorf("unexpected badge path %q", path)
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, "", fmt.Errorf("unexpected pull request number in badge path %q: %w", path, err)
	}
	return id, parts[2], nil
}

// scheduleReport asks for the report covering badgePath to be rebuilt.  It
// returns immediately; the rebuild happens on its own goroutine after
// reportCoalesceDelay so that sibling results can be absorbed into it.
func scheduleReport(badgePath string) {
	id, headSHA, err := parseBadgePath(badgePath)
	if err != nil {
		glog.Errorf("Skipping report refresh: %s", err)
		return
	}
	// Keyed by commit, not just by pull request: results for a superseded
	// commit must not be coalesced with results for the current one.
	key := strconv.Itoa(id) + "/" + headSHA

	reportRefresh.mu.Lock()
	defer reportRefresh.mu.Unlock()
	if reportRefresh.inFlight[key] {
		// A rebuild is already pending.  Flag it so that, if this result
		// arrived too late for that rebuild to see, it runs again.
		reportRefresh.dirty[key] = true
		return
	}
	reportRefresh.inFlight[key] = true
	// Safe to start under the lock: the goroutine sleeps before it locks.
	go runReportRefresh(key, id, headSHA)
}

// runReportRefresh waits for sibling results to land, rebuilds the report,
// and repeats while further results arrive.  Exactly one of these runs per
// commit at a time, which is what keeps concurrent rebuilds from racing.
func runReportRefresh(key string, id int, headSHA string) {
	for {
		time.Sleep(reportCoalesceDelay)
		refreshReport(id, headSHA)

		reportRefresh.mu.Lock()
		if reportRefresh.dirty[key] {
			// A result landed while the rebuild above was running and may
			// not be in the report.  Clear the flag and rebuild again.
			delete(reportRefresh.dirty, key)
			reportRefresh.mu.Unlock()
			continue
		}
		delete(reportRefresh.inFlight, key)
		reportRefresh.mu.Unlock()
		return
	}
}

// refreshReport rebuilds the report comment for one commit.  All failures
// are logged and swallowed: a report that is briefly out of date is far
// better than losing badge updates or taking down the webhook server, which
// shares this process.
func refreshReport(id int, headSHA string) {
	// identifyModifiedTests clones and walks a contributor's branch.  The
	// webhook path runs the same code, but this path runs it far more
	// often, so contain a panic here rather than let it stop the process.
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("Recovered while refreshing report for PR%d commit %q: %v", id, headSHA, r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), reportTimeout)
	defer cancel()

	// Clients are built per rebuild rather than once at start up for two
	// reasons: newTrigger re-reads the GitHub API secret, so a rotated
	// secret is picked up the way it is on the webhook path; and a failure
	// here stays local instead of aborting a process that also serves the
	// GitHub webhook.
	t, err := newTrigger(ctx)
	if err != nil {
		glog.Errorf("Skipping report refresh for PR%d: setup error: %s", id, err)
		return
	}
	defer func() {
		if t.pubsubClient != nil {
			t.pubsubClient.Close()
		}
		if t.storClient != nil {
			t.storClient.Close()
		}
	}()

	if err := t.updateReport(ctx, id, headSHA); err != nil {
		glog.Errorf("Failed to refresh report for PR%d commit %q: %s", id, headSHA, err)
	}
}

// updateReport rewrites the pull request report comment so that it shows the
// current badge status.  It reads the same object metadata the webhook path
// reads, so the report it produces is identical to the one a push or an
// "/fptest" comment would produce at this moment.
func (t *trigger) updateReport(ctx context.Context, id int, headSHA string) error {
	prData, _, err := t.githubClient.PullRequests.Get(ctx, githubProjectOwner, githubProjectRepo, id)
	if err != nil {
		return fmt.Errorf("query GitHub API for PR data: %w", err)
	}

	// A newer commit may have been pushed while these tests were running.
	// Its report describes the new commit, so leave it alone rather than
	// overwrite it with results belonging to the superseded one.
	if prData.GetHead().GetSHA() != headSHA {
		glog.Infof("Skipping report update for PR%d commit %q: a newer commit has been pushed", id, headSHA)
		return nil
	}

	// Refresh an existing report only; never create one.
	//
	// updateGitHub creates a comment when firstComment finds none, and
	// firstComment reads only the first page of comments.  On a pull
	// request with a long discussion it can therefore miss a report that
	// does exist, and creating here would leave two reports on the pull
	// request.  Creating the report is the webhook path's job; this path
	// only keeps it current, so bailing out is always the safe choice.
	existing, err := (&pullRequest{ID: id}).firstComment(ctx, t.githubClient)
	if err != nil {
		return fmt.Errorf("list comments: %w", err)
	}
	if existing == nil {
		glog.Infof("Skipping report update for PR%d: no existing report to refresh", id)
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "fptest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Rebuild the same pullRequest the webhook path would build.  The
	// pubsub message carries only a badge path, so the clone URL comes from
	// the API and the test list is recomputed from the checked out tree.
	pr := &pullRequest{
		ID:        id,
		HeadSHA:   headSHA,
		cloneURL:  prData.GetHead().GetRepo().GetCloneURL(),
		localFS:   os.DirFS(tmpDir),
		localPath: tmpDir,
	}
	if err := pr.identifyModifiedTests(); err != nil {
		return fmt.Errorf("identify modified tests for commit %q: %w", headSHA, err)
	}
	// Overlays the real per-test status recorded on the badge objects.
	pr.populateObjectMetadata(ctx, t.storClient)

	return pr.updateGitHub(ctx, t.githubClient)
}

// pullSubscription subscribes to the gcpBadgeTopic on gcpProjectID and
// processes the messages.
func pullSubscription() {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		glog.Fatalf("Failed creating pubsub client: %s", err)
	}
	defer client.Close()

	sub := client.Subscription(gcpBadgeTopic)
	err = sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
		msg.Ack()
		bs := &badgeState{}
		if err := json.Unmarshal(msg.Data, bs); err != nil {
			glog.Errorf("Failed to decode subscription message %q: %s", msg.Data, err)
			return
		}
		if err := updateBadgeStatus(ctx, bs); err != nil {
			glog.Errorf("Failed to update badge state: %s", err)
			return
		}
		// Everything above is unchanged.  Only a finished test changes what
		// the report should say, so only those schedule a rebuild.  Pubsub
		// delivers at least once, but a rebuild simply re-reads the badge
		// metadata, so a duplicate message is harmless.
		if !terminalTestStatus[bs.Status] {
			return
		}
		scheduleReport(bs.Path)
	})
	if err != nil {
		glog.Fatalf("Failed receiving pubsub message: %s", err)
	}
}
