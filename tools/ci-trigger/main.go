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

// ci-trigger is a Google Cloud Run container that manages FeatureProfiles CI
// events.  The Cloud Run container uses the GitHub API to inspect pull requests
// and identify changes.  If a pull request changes Ondatra tests, an authorized
// user can comment in the pull request to cause CI Trigger to launch a Google
// Cloud Build task to validate tests on various virtual/hardware platforms.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v50/github"

	"github.com/golang/glog"
)

var badgePubsub = flag.Bool("badge_pubsub", true, "Process badge pubsub events")

func main() {
	flag.Parse()
	glog.Info("Starting server...")
	http.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		event, err := parseEvent(r)
		if err != nil {
			glog.Errorf("parseEvent error: %s", err)
			return
		}

		go processEvent(event)
	})

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		glog.Infof("Defaulting to port %s", port)
	}

	if *badgePubsub {
		go pullSubscription()
	}

	// Start HTTP server.
	glog.Infof("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		glog.Fatal(err)
	}
}

// processEvent handles a GitHub Webhook event.
func processEvent(event any) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t, err := newTrigger(ctx)
	if err != nil {
		glog.Errorf("Setup error: %s", err)
		return
	}

	switch event := event.(type) {
	case *github.PullRequestEvent:
		if event.GetAction() == "opened" || event.GetAction() == "synchronize" {
			if err := t.processPullRequest(ctx, event); err != nil {
				glog.Errorf("ProcessPullRequest PR%d user %q error: %s", event.GetPullRequest().GetNumber(), event.GetPullRequest().GetUser().GetLogin(), err)
				return
			}
		}
	case *github.IssueCommentEvent:
		if event.GetAction() == "created" {
			if err := t.processIssueComment(ctx, event); err != nil {
				glog.Errorf("ProcessIssueComment PR%d comment %d user %q error: %s", event.GetIssue().GetNumber(), event.GetComment().GetID(), event.GetComment().GetUser().GetLogin(), err)
				return
			}
		}
	}
}

// parseEvent returns the validated Github event from an HTTP request.
func parseEvent(r *http.Request) (any, error) {
	webhookSecret, err := fetchWebhookSecret()
	if err != nil {
		return nil, err
	}

	payload, err := github.ValidatePayload(r, webhookSecret)
	if err != nil {
		return nil, err
	}

	return github.ParseWebHook(github.WebHookType(r), payload)
}

// fetchWebhookSecret returns the webhookSecret for the running environment.
func fetchWebhookSecret() ([]byte, error) {
	var webhookSecret []byte
	if envWebhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET"); envWebhookSecret != "" {
		webhookSecret = []byte(envWebhookSecret)
	} else {
		var err error
		webhookSecret, err = os.ReadFile("/etc/secrets/github-webhook-secret/github-webhook-secret")
		if err != nil {
			return nil, err
		}
	}

	return webhookSecret, nil
}

// fetchAPISecret returns the apiSecret for the running environment.
func fetchAPISecret() ([]byte, error) {
	var apiSecret []byte
	if envAPISecret := os.Getenv("GITHUB_API_SECRET"); envAPISecret != "" {
		apiSecret = []byte(envAPISecret)
	} else {
		var err error
		apiSecret, err = os.ReadFile("/etc/secrets/github-api-secret/github-api-secret")
		if err != nil {
			return nil, err
		}
	}
	return apiSecret, nil
}
