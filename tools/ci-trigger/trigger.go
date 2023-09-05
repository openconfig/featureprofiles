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
	"fmt"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
	"google.golang.org/api/cloudbuild/v1"
)

// trigger contains the functions used to process a GitHub Webhook event
type trigger struct {
	githubClient *github.Client
	pubsubClient *pubsub.Client
	storClient   *storage.Client
	buildClient  *cloudbuild.Service
}

// processIssueComment handles a GitHub issue event.
func (t *trigger) processIssueComment(ctx context.Context, e *github.IssueCommentEvent) error {
	// Skip processing if the issue comment is not related to a pull request.
	if e.GetIssue().GetPullRequestLinks().GetURL() == "" {
		return nil
	}

	requestingUser := e.GetComment().GetUser().GetLogin()
	auth, err := t.authorizedUser(ctx, requestingUser)
	if err != nil {
		return fmt.Errorf("validate user %q auth: %w", requestingUser, err)
	}
	if !auth {
		return nil
	}

	containsTriggerKeyword := false
	for keyword := range triggerKeywords {
		if strings.Contains(strings.ToLower(e.GetComment().GetBody()), keyword) {
			containsTriggerKeyword = true
			break
		}
	}
	if !containsTriggerKeyword {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "fptest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	prData, _, err := t.githubClient.PullRequests.Get(ctx, githubProjectOwner, githubProjectRepo, e.GetIssue().GetNumber())
	if err != nil {
		return fmt.Errorf("query GitHub API for PR data: %w", err)
	}

	pr := &pullRequest{
		ID:        e.GetIssue().GetNumber(),
		HeadSHA:   prData.GetHead().GetSHA(),
		cloneURL:  prData.GetHead().GetRepo().GetCloneURL(),
		localFS:   os.DirFS(tmpDir),
		localPath: tmpDir,
	}
	if err := pr.identifyModifiedTests(); err != nil {
		return fmt.Errorf("identify modified tests for commit %q: %w", pr.HeadSHA, err)
	}

	pr.populateObjectMetadata(ctx, t.storClient)

	for keyword, deviceTypes := range triggerKeywords {
		if strings.Contains(strings.ToLower(e.GetComment().GetBody()), keyword) {
			glog.Infof("User %q launching test jobs for PR%d at commit %q", requestingUser, pr.ID, pr.HeadSHA)
			if err := pr.createBuild(ctx, t.buildClient, t.storClient, t.pubsubClient, deviceTypes); err != nil {
				return fmt.Errorf("create build for commit %q: %w", pr.HeadSHA, err)
			}

			if err := pr.updateBadges(ctx, t.storClient); err != nil {
				return fmt.Errorf("update GCS badges for commit %q: %w", pr.HeadSHA, err)
			}

			if err := pr.updateGitHub(ctx, t.githubClient); err != nil {
				return fmt.Errorf("update GitHub PR for commit %q: %w", pr.HeadSHA, err)
			}

			break
		}
	}

	return nil
}

// processPullRequest handles a GitHub Pull Request event.
func (t *trigger) processPullRequest(ctx context.Context, e *github.PullRequestEvent) error {
	tmpDir, err := os.MkdirTemp("", "fptest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	pr := &pullRequest{
		ID:        e.GetPullRequest().GetNumber(),
		HeadSHA:   e.GetPullRequest().GetHead().GetSHA(),
		cloneURL:  e.GetPullRequest().GetHead().GetRepo().GetCloneURL(),
		localFS:   os.DirFS(tmpDir),
		localPath: tmpDir,
	}
	if err := pr.identifyModifiedTests(); err != nil {
		return fmt.Errorf("identify modified tests for commit %q: %w", pr.HeadSHA, err)
	}

	requestingUser := e.GetPullRequest().GetUser().GetLogin()
	auth, err := t.authorizedUser(ctx, requestingUser)
	if err != nil {
		return fmt.Errorf("validate user %q auth: %w", requestingUser, err)
	}
	if auth {
		glog.Infof("User %q launching test jobs for PR%d at commit %q", requestingUser, pr.ID, pr.HeadSHA)
		if err := pr.createBuild(ctx, t.buildClient, t.storClient, t.pubsubClient, virtualDeviceTypes); err != nil {
			return fmt.Errorf("create build for commit %q: %w", pr.HeadSHA, err)
		}
	}

	if err := pr.updateBadges(ctx, t.storClient); err != nil {
		return fmt.Errorf("update GCS badges for commit %q: %w", pr.HeadSHA, err)
	}

	if err := pr.updateGitHub(ctx, t.githubClient); err != nil {
		return fmt.Errorf("update GitHub PR for commit %q: %w", pr.HeadSHA, err)
	}

	return nil
}

// authorizedUser checks the GitHub API to see if the user is a member of an authorizedTeams.
func (t *trigger) authorizedUser(ctx context.Context, username string) (bool, error) {
	for _, authorizedTeam := range authorizedTeams {
		result, _, err := t.githubClient.Teams.GetTeamMembershipBySlug(ctx, githubProjectOwner, authorizedTeam, username)
		if err != nil {
			// StatusNotFound is returned when the user is not a member of the group.
			if err, ok := err.(*github.ErrorResponse); ok && err.Response.StatusCode == http.StatusNotFound {
				continue
			}
			return false, err
		}
		if result.GetState() == "active" {
			return true, nil
		}
	}

	return false, nil
}

func newTrigger(ctx context.Context) (*trigger, error) {
	t := &trigger{}

	apiSecret, err := fetchAPISecret()
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(apiSecret)},
	)
	tc := oauth2.NewClient(ctx, ts)
	t.githubClient = github.NewClient(tc)
	t.pubsubClient, err = pubsub.NewClient(ctx, gcpProjectID)
	if err != nil {
		return nil, err
	}
	t.storClient, err = storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	t.buildClient, err = cloudbuild.NewService(ctx)
	return t, err
}
