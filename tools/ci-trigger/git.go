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
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// setupGitClone clones the GitHub repository into tmpDir and fetches refs from remoteURL.
func setupGitClone(tmpDir, remoteURL, head string) (*git.Repository, error) {
	repo, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL: "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + ".git",
	})
	if err != nil {
		return nil, err
	}

	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "pr",
		URLs: []string{remoteURL},
	})
	if err != nil {
		return nil, err
	}

	if err := remote.Fetch(&git.FetchOptions{}); err != nil {
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	if err := wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(head)}); err != nil {
		return nil, err
	}

	return repo, nil
}

// modifiedFiles returns a list of files affected for any reason between the head and base SHA hashes.
func modifiedFiles(repo *git.Repository, head, base string) ([]string, error) {
	headCommit, err := repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return nil, err
	}
	baseCommit, err := repo.CommitObject(plumbing.NewHash(base))
	if err != nil {
		return nil, err
	}

	patch, err := baseCommit.Patch(headCommit)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, p := range patch.FilePatches() {
		from, to := p.Files()
		if from != nil {
			result = append(result, from.Path())
		}
		if to != nil {
			result = append(result, to.Path())
		}
	}
	return result, nil
}
