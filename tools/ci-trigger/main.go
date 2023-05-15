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
	"flag"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/google/go-github/v50/github"
)

func main() {
	flag.Parse()
	glog.Info("Starting server...")
	http.HandleFunc("/", ghWebhook)

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		glog.Infof("Defaulting to port %s", port)
	}

	go pullSubscription()

	// Start HTTP server.
	glog.Infof("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		glog.Fatal(err)
	}
}

func ghWebhook(_ http.ResponseWriter, r *http.Request) {
	t, err := newTrigger(r.Context())
	if err != nil {
		glog.Errorf("Setup error: %s", err)
		return
	}

	event, err := t.githubEvent(r)
	if err != nil {
		glog.Errorf("GithubEvent error: %s", err)
		return
	}

	switch event := event.(type) {
	case *github.PullRequestEvent:
		if event.GetAction() == "opened" || event.GetAction() == "synchronize" {
			if err := t.processPullRequest(r.Context(), event); err != nil {
				glog.Errorf("ProcessPullRequest error: %s", err)
				return
			}
		}
	case *github.IssueCommentEvent:
		if event.GetAction() == "created" {
			if err := t.processIssueComment(r.Context(), event); err != nil {
				glog.Errorf("ProcessIssueComment error: %s", err)
				return
			}
		}
	}
}

// fetchSecrets returns the webhookSecret and apiSecret for the
// running environment.
func fetchSecrets() ([]byte, []byte, error) {
	var err error
	var webhookSecret, apiSecret []byte
	if envWebhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET"); envWebhookSecret != "" {
		webhookSecret = []byte(envWebhookSecret)
	} else {
		webhookSecret, err = os.ReadFile("/etc/secrets/github-webhook-secret/github-webhook-secret")
		if err != nil {
			return nil, nil, err
		}
	}

	if envAPISecret := os.Getenv("GITHUB_API_SECRET"); envAPISecret != "" {
		apiSecret = []byte(envAPISecret)
	} else {
		apiSecret, err = os.ReadFile("/etc/secrets/github-api-secret/github-api-secret")
		if err != nil {
			return nil, nil, err
		}
	}

	return webhookSecret, apiSecret, nil
}
