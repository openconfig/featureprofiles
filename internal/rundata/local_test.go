// Copyright 2022 Google LLC
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

package rundata

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	gitV5 "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-cmp/cmp"
)

func TestBuildInfo(t *testing.T) {
	if _, ok := debug.ReadBuildInfo(); !ok {
		t.Skip("Skipping test because the runtime environment is missing BuildInfo.")
	}

	m := make(map[string]string)
	buildInfo(m)
	t.Log(m)

	for _, k := range []string{
		"build.go_version",
		"build.path",
		"build.main.path",
		"build.main.version",
		"build.main.sum",
	} {
		if _, ok := m[k]; !ok {
			t.Errorf("Missing key from buildInfo: %s", k)
		}
	}
}

func newGitRepo() (*gitV5.Repository, error) {
	// Use repo.Storer to get the object storer, and
	// repo.Worktree().Filesystem to get the worktree.
	return gitV5.Init(
		memory.NewStorage(),
		memfs.New(),
	)
}

const (
	wantOrigin = "https://git.example.com/origin.git"
	pushOrigin = "https://git.example.com/push.git"
)

var originConfig = &config.RemoteConfig{
	Name: "origin",
	URLs: []string{wantOrigin, pushOrigin},
}

var commitSignature = &object.Signature{
	Name:  "Alan Smithee",
	Email: "alan.smithee@example.com",
	When:  time.Now().Round(time.Second), // Git only keeps time in seconds.
}

func addCommit(repo *gitV5.Repository) (plumbing.Hash, error) {
	var emptyHash plumbing.Hash

	wt, err := repo.Worktree()
	if err != nil {
		return emptyHash, err
	}
	fs := wt.Filesystem

	f, err := fs.Create("foo")
	if err != nil {
		return emptyHash, err
	}
	f.Write([]byte("the quick brown fox jumps over a lazy dog\n"))
	f.Close()

	wt.Add("foo")
	return wt.Commit("commit message", &gitV5.CommitOptions{
		Author: commitSignature,
	})
}

func TestGitOrigin(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}

	_, err = repo.CreateRemote(originConfig)
	if err != nil {
		t.Fatalf("Could not add remote: %v", err)
	}

	got, err := gitOrigin(repo)
	if err != nil {
		t.Fatalf("Could not get origin: %v", err)
	}
	if got != wantOrigin {
		t.Errorf("gitOrigin got %q, want %q", got, wantOrigin)
	}
}

func TestGitOrigin_NoOrigin(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}

	_, err = gitOrigin(repo)
	t.Logf("gitOrigin got error: %v", err)
	if err == nil {
		t.Errorf("gitOrigin got nil error, want != nil")
	}
}

func TestGitHead(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}

	want, err := addCommit(repo)
	if err != nil {
		t.Fatalf("Could not create commit: %v", err)
	}

	got, gotWhen, err := gitHead(repo)
	if err != nil {
		t.Fatalf("Could not get gitHead: %v", err)
	}
	if got != want.String() {
		t.Errorf("Commit hash got %q, want %q", got, want)
	}
	wantWhen := commitSignature.When
	if gotWhen.UTC() != wantWhen.UTC() {
		t.Errorf("Commit time got %v, want %v", gotWhen, wantWhen)
	}
}

func TestGitHead_NoHead(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}

	_, _, err = gitHead(repo)
	t.Logf("gitHead got error: %v", err)
	if err == nil {
		t.Errorf("gitHead got nil error, want != nil")
	}
}

func TestGitInfoWithRepo(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}
	_, err = repo.CreateRemote(originConfig)
	if err != nil {
		t.Fatalf("Could not add remote: %v", err)
	}
	wantCommit, err := addCommit(repo)
	if err != nil {
		t.Fatalf("Could not create commit: %v", err)
	}

	want := map[string]string{
		"git.clean":            "true",
		"git.commit":           wantCommit.String(),
		"git.commit_timestamp": fmt.Sprint(commitSignature.When.Unix()),
		"git.origin":           wantOrigin,
		"git.status":           "",
	}

	got := make(map[string]string)
	gitInfoWithRepo(got, repo)
	t.Log(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("gitInfoWithRepo -want, +got:\n%s", diff)
	}
}

func TestGitInfoWithRepo_NotClean(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}
	_, err = repo.CreateRemote(originConfig)
	if err != nil {
		t.Fatalf("Could not add remote: %v", err)
	}
	wantCommit, err := addCommit(repo)
	if err != nil {
		t.Fatalf("Could not create commit: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Could not get work tree: %v", err)
	}
	fs := wt.Filesystem

	f, err := fs.Create("bar")
	if err != nil {
		t.Fatalf("Could not create fake file: %v", err)
	}
	f.Write([]byte("grumpy wizards make toxic brew for the evil queen and jack\n"))
	f.Close()

	want := map[string]string{
		"git.clean":            "false",
		"git.commit":           wantCommit.String(),
		"git.commit_timestamp": fmt.Sprint(commitSignature.When.Unix()),
		"git.origin":           wantOrigin,
		"git.status":           "?? bar\n",
	}

	got := make(map[string]string)
	gitInfoWithRepo(got, repo)
	t.Log(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("gitInfoWithRepo -want, +got:\n%s", diff)
	}
}

func TestGitInfoWithRepo_NoOriginHead(t *testing.T) {
	repo, err := newGitRepo()
	if err != nil {
		t.Fatalf("Could not create git repo: %v", err)
	}

	want := map[string]string{
		"git.clean":  "true",
		"git.status": "",
	}

	got := make(map[string]string)
	gitInfoWithRepo(got, repo)
	t.Log(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("gitInfoWithRepo -want, +got:\n%s", diff)
	}
}

// oneLineOutput runs a command cmd and returns its one line output
// where the trailing newline is removed.  The command should be only
// space delimited (no shell quoting).
func oneLineOutput(cmd string) (string, error) {
	argv := strings.Split(cmd, " ")
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// mapToOutput runs the commands from the input map values and returns
// a new map where the values are the outputs of these commands.
func mapToOutput(cmds map[string]string) (map[string]string, error) {
	outs := make(map[string]string)
	for k, cmd := range cmds {
		v, err := oneLineOutput(cmd)
		if err != nil {
			return nil, err
		}
		outs[k] = v
	}
	return outs, nil
}

func TestGitInfo(t *testing.T) {
	wantWd, err := oneLineOutput("git rev-parse --show-toplevel")
	if err != nil {
		t.Skipf("Skipping test because either we could not run git or we are not running under a git repository: %v", err)
	}

	want, err := mapToOutput(map[string]string{
		"git.commit":           "git show -s --format=%H",
		"git.commit_timestamp": "git show -s --format=%ct",
		"git.origin":           "git config --get remote.origin.url",
		"git.status":           "git status --short",
	})
	if err != nil {
		t.Fatalf("Could not run git to get the source of truth: %v", err)
	}

	if want["git.status"] == "" {
		want["git.clean"] = "true"
	} else {
		want["git.clean"] = "false"
	}

	got := make(map[string]string)
	gotWd := gitInfo(got)
	if gotWd != wantWd {
		t.Errorf("gitInfo got %q, want %q", gotWd, wantWd)
	}
	t.Log(got)

	if got["git.status"] != "" {
		// The output formats differ slightly.
		got["git.status"] = want["git.status"]
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("gitInfo -want, +got:\n%s", diff)
	}
}

const wantTestPath = "internal/rundata"

func TestTestPath(t *testing.T) {
	got := testPath("")
	if got != wantTestPath {
		t.Errorf("testPath got %q, want %q", got, wantTestPath)
	}
}

func TestTestPath_FromGit(t *testing.T) {
	wd, err := oneLineOutput("git rev-parse --show-toplevel")
	if err != nil {
		t.Skipf("Skipping test because either we could not run git or we are not running under a git repository: %v", err)
	}
	got := testPath(wd)
	if got != wantTestPath {
		t.Errorf("testPath got %q, want %q", got, wantTestPath)
	}
}

func TestDeviationInfo(t *testing.T) {
	cases := []struct {
		name string
		desc string
		args []string
		want map[string]string
	}{{
		name: "Empty",
		desc: "No deviation flag specified should result in the empty map.",
		args: []string{},
		want: map[string]string{},
	}, {
		name: "DefaultValues",
		desc: "Explicitly given default values should result in the empty map.",
		args: []string{
			"-deviation_foo=false",
			"-deviation_bar=DEFAULT",
			"-deviation_qux=42",
		},
		want: map[string]string{},
	}, {
		name: "Deviated",
		desc: "Only the deviations should be enumerated.",
		args: []string{
			"-deviation_foo",
			"-deviation_bar=NOT_DEFAULT",
			// no -deviation_qux
			"-xyzzy=opensesame", // not a deviation.
		},
		want: map[string]string{
			"deviation.foo": "true",
			"deviation.bar": "NOT_DEFAULT",
		},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Log(c.desc)

			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Bool("deviation_foo", false, "foo is a bool deviation")
			fs.String("deviation_bar", "DEFAULT", "bar is a string deviation")
			fs.Int("deviation_qux", 42, "qux is an int deviation")
			fs.String("xyzzy", "letmein", "xyzzy is not a deviation")

			fs.Parse(c.args)
			got := make(map[string]string)
			deviationInfo(got, fs)
			t.Log(got)

			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("deviationInfo -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLocal(t *testing.T) {
	m := make(map[string]string)
	local(m)
	t.Log(m)

	if _, ok := m["test.path"]; !ok {
		t.Errorf("Missing test.path key from local.")
	}
}

// initCLIRepo creates a git repository with one commit in a temporary
// directory using the git CLI, passing initArgs to git init. It skips
// the test if git is unavailable or does not support initArgs.
func initCLIRepo(t *testing.T, initArgs ...string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		// Isolate from the user's and system's git configuration.
		cmd.Env = append(cmd.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v\n%s", args, err, out)
		}
		return nil
	}
	if err := git(append([]string{"init", "-q"}, initArgs...)...); err != nil {
		t.Skipf("Skipping test because git init %v is not supported: %v", initArgs, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo"), []byte("foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"remote", "add", "origin", wantOrigin},
		{"add", "foo"},
		{"-c", "user.name=Alan Smithee", "-c", "user.email=alan.smithee@example.com", "commit", "-q", "-m", "commit message"},
	} {
		if err := git(args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGitInfo_Reftable(t *testing.T) {
	dir := initCLIRepo(t, "--ref-format=reftable")
	wantWd, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantCommit, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	got := make(map[string]string)
	if gotWd := gitInfo(got); gotWd != wantWd {
		t.Errorf("gitInfo got working directory %q, want %q", gotWd, wantWd)
	}
	if got["git.commit"] != wantCommit {
		t.Errorf("gitInfo got git.commit %q, want %q", got["git.commit"], wantCommit)
	}
	if got["git.origin"] != wantOrigin {
		t.Errorf("gitInfo got git.origin %q, want %q", got["git.origin"], wantOrigin)
	}
	if got["git.commit_timestamp"] == "" {
		t.Error("gitInfo got empty git.commit_timestamp")
	}
	if got["git.clean"] != "true" {
		t.Errorf("gitInfo got git.clean %q, want \"true\"", got["git.clean"])
	}

	if err := os.WriteFile(filepath.Join(dir, "foo"), []byte("bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = make(map[string]string)
	gitInfo(got)
	if got["git.clean"] != "false" {
		t.Errorf("gitInfo after modifying a tracked file got git.clean %q, want \"false\"", got["git.clean"])
	}
	if got["git.status"] == "" {
		t.Error("gitInfo after modifying a tracked file got empty git.status")
	}
}

func TestGitInfo_NotRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	got := make(map[string]string)
	if gotWd := gitInfo(got); gotWd != "" {
		t.Errorf("gitInfo got working directory %q, want \"\"", gotWd)
	}
	if len(got) != 0 {
		t.Errorf("gitInfo got %v, want no properties", got)
	}
}

func TestGitInfoWithCLI_NoGit(t *testing.T) {
	orig := runGit
	t.Cleanup(func() { runGit = orig })
	runGit = func(string, ...string) (string, error) {
		return "", errors.New("git not found")
	}
	got := make(map[string]string)
	if gotWd := gitInfoWithCLI(got, t.TempDir()); gotWd != "" {
		t.Errorf("gitInfoWithCLI got working directory %q, want \"\"", gotWd)
	}
	if len(got) != 0 {
		t.Errorf("gitInfoWithCLI got %v, want no properties", got)
	}
}

func TestGitProperties(t *testing.T) {
	dir := initCLIRepo(t)
	t.Chdir(dir)
	got := GitProperties()
	for _, k := range []string{"git.commit", "git.commit_timestamp", "git.origin", "git.clean"} {
		if got[k] == "" {
			t.Errorf("GitProperties() missing %q, got %v", k, got)
		}
	}
	if _, ok := got["build.go_version"]; ok {
		t.Errorf("GitProperties() got non-git property build.go_version")
	}
}
