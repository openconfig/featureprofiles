package main

import (
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

func createCommit(wt *git.Worktree, contents string) error {
	const fileName = "foo.txt"
	author := object.Signature{
		Name:  "go-git",
		Email: "go-git@fake.local",
		When:  time.Now(),
	}

	rm, err := wt.Filesystem.Create(fileName)
	if err != nil {
		return err
	}

	if _, err := rm.Write([]byte(contents)); err != nil {
		return err
	}

	if _, err := wt.Add(fileName); err != nil {
		return err
	}

	_, err = wt.Commit("test commit message", &git.CommitOptions{
		All:       true,
		Author:    &author,
		Committer: &author,
	})
	return err
}

func createBranch(r *git.Repository, name string) (*plumbing.Reference, error) {
	branchRef := plumbing.NewBranchReferenceName(name)
	headRef, err := r.Head()
	if err != nil {
		return nil, err
	}

	ref := plumbing.NewHashReference(branchRef, headRef.Hash())
	if err := r.Storer.SetReference(ref); err != nil {
		return nil, err
	}

	return ref, nil
}

func createBaseRepo() (*git.Repository, *git.Worktree, error) {
	r, err := git.InitWithOptions(memory.NewStorage(), memfs.New(), git.InitOptions{DefaultBranch: plumbing.Main})
	if err != nil {
		return nil, nil, err
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, nil, err
	}

	if err := createCommit(wt, "test commit before"); err != nil {
		return nil, nil, err
	}

	return r, wt, err
}

func headCommitSHA(r *git.Repository) (string, error) {
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func TestMergeBase(t *testing.T) {
	r, wt, err := createBaseRepo()
	if err != nil {
		t.Fatalf("Failed to create base repo: %s", err)
	}

	baseCommitSHA, err := headCommitSHA(r)
	if err != nil {
		t.Fatalf("Failed to fetch base commit head ref: %s", err)
	}

	ref, err := createBranch(r, "new-branch")
	if err != nil {
		t.Fatalf("Failed to create branch: %s", err)
	}

	if err := createCommit(wt, "test commit after"); err != nil {
		t.Fatalf("Failed to create commit: %s", err)
	}

	if err := wt.Checkout(&git.CheckoutOptions{Branch: ref.Name()}); err != nil {
		t.Fatalf("Failed to checkout branch: %s", err)
	}

	if err := createCommit(wt, "test commit in branch"); err != nil {
		t.Fatalf("Failed to create commit in branch: %s", err)
	}

	headCommitSHA, err := headCommitSHA(r)
	if err != nil {
		t.Fatalf("Failed to fetch head commit head ref: %s", err)
	}

	res, err := mergeBase(r, "main", headCommitSHA)
	if res != baseCommitSHA {
		t.Errorf("mergeBase(): unexpected SHA: got %s, want %s", res, baseCommitSHA)
	}
	if err != nil {
		t.Errorf("mergeBase(): unexpected error: %s", err)
	}
}
