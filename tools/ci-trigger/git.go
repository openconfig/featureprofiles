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
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// modifiedFiles returns a list of files affected for any reason between the head and main branch.
func modifiedFiles(repo *git.Repository, head string) ([]string, error) {
	headCommit, err := repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return nil, err
	}

	base, err := mergeBase(repo, head, "origin/main")
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

// mergeBase returns the common ancestor hash from the head and base commits.
func mergeBase(repo *git.Repository, head, base string) (string, error) {
	var hashes []*plumbing.Hash
	for _, rev := range []string{head, base} {
		hash, err := repo.ResolveRevision(plumbing.Revision(rev))
		if err != nil {
			return "", fmt.Errorf("could not parse revision '%s': %w", rev, err)
		}
		hashes = append(hashes, hash)
	}

	var commits []*object.Commit
	for _, hash := range hashes {
		commit, err := repo.CommitObject(*hash)
		if err != nil {
			return "", fmt.Errorf("could not find commit '%s': %w", hash.String(), err)
		}
		commits = append(commits, commit)
	}

	res, err := commits[0].MergeBase(commits[1])
	if err != nil {
		return "", fmt.Errorf("could not traverse the repository history: %w", err)
	}

	if len(res) < 1 {
		return "", fmt.Errorf("no common git ancestor found for hash %q", head)
	}

	return res[0].Hash.String(), nil
}
