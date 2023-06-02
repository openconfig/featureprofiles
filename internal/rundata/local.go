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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"flag"

	gitv5 "github.com/go-git/go-git/v5"
	"github.com/golang/glog"
)

// buildInfo populates the properties from debug.ReadBuildInfo.
func buildInfo(m map[string]string) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		glog.Warning("debug.ReadBuildInfo() returned no BuildInfo.")
		return
	}
	m["build.go_version"] = bi.GoVersion
	m["build.path"] = bi.Path
	m["build.main.path"] = bi.Main.Path
	m["build.main.version"] = bi.Main.Version
	m["build.main.sum"] = bi.Main.Sum

	for _, setting := range bi.Settings {
		m[fmt.Sprintf("build.settings.%s", setting.Key)] = setting.Value
	}
}

// gitOrigin returns the fetch URL of the "origin" remote.
func gitOrigin(repo *gitv5.Repository) (string, error) {
	origin, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}
	config := origin.Config()
	if len(config.URLs) == 0 {
		return "", errors.New("origin has no URLs")
	}
	return config.URLs[0], nil // First one is always used for fetching.
}

// gitHead returns the commit hash and the commit timestamp at HEAD.
func gitHead(repo *gitv5.Repository) (string, time.Time, error) {
	var zero time.Time
	head, err := repo.Head()
	if err != nil {
		return "", zero, err
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", zero, err
	}
	return commit.Hash.String(), commit.Committer.When, nil
}

// gitInfoWithRepo populates the git properties from a given git repo
// and returns the path to the working directory.
func gitInfoWithRepo(m map[string]string, repo *gitv5.Repository) string {
	wt, err := repo.Worktree()
	if err != nil {
		return ""
	}

	if origin, err := gitOrigin(repo); err != nil {
		glog.Warningf("Could not get git origin URL: %v", err)
	} else {
		m["git.origin"] = origin
	}

	if commitHash, commitTime, err := gitHead(repo); err != nil {
		glog.Warningf("Could not get git HEAD: %v", err)
	} else {
		m["git.commit"] = commitHash
		m["git.commit_timestamp"] = fmt.Sprint(commitTime.Unix())
	}

	if status, err := wt.Status(); err != nil {
		glog.Warningf("Could not get git status: %v", err)
	} else {
		m["git.status"] = status.String()
		if status.IsClean() {
			m["git.clean"] = "true"
		} else {
			m["git.clean"] = "false"
		}
	}
	return wt.Filesystem.Root()
}

// gitInfo populates the git properties from the current working
// directory as the workspace and returns the path to the working
// directory.
func gitInfo(m map[string]string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	repo, err := gitv5.PlainOpenWithOptions(cwd, &gitv5.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return ""
	}
	return gitInfoWithRepo(m, repo)
}

// fpPath returns the package path of a test file path under the
// featureprofiles repo.
func fpPath(testpath string) string {
	const part = "/featureprofiles/"
	i := strings.LastIndex(testpath, part)
	if i < 0 {
		return ""
	}
	i += len(part)
	j := strings.LastIndexByte(testpath, '/')
	if j < 0 || j < i {
		return ""
	}
	return testpath[i:j]
}

// testPath detects the relative path of the test to the base of the
// repo.  If we are running in a git working tree, wd will be used as
// the base.  If we are not running in a git working tree (wd is
// empty), the test path will be the portion of the package path of
// the test function after "featureprofiles".
//
// In both cases, the test path is obtained by traversing the call
// stack until we find a caller from a file matching the "_test.go"
// suffix.
func testPath(wd string) string {
	var callers [32]uintptr
	n := runtime.Callers(0, callers[:])
	frames := runtime.CallersFrames(callers[:n])

	var frame runtime.Frame
	var more bool
	for {
		frame, more = frames.Next()
		if !more {
			return ""
		}
		if strings.HasSuffix(frame.File, "_test.go") {
			break
		}
	}

	if wd == "" {
		return fpPath(frame.File)
	}

	dir := filepath.Dir(frame.File)
	prefix := filepath.Clean(wd) + "/"
	if strings.HasPrefix(dir, prefix) {
		return dir[len(prefix):]
	}
	return ""
}

// deviationInfo populates all deviation flags that have non-default
// values by visiting flags with the "deviation_" prefix.
func deviationInfo(m map[string]string, fs *flag.FlagSet) {
	fs.Visit(func(f *flag.Flag) {
		const prefix = "deviation_"
		if !strings.HasPrefix(f.Name, prefix) {
			return
		}
		value := f.Value.String()
		if f.DefValue == value {
			return
		}
		name := f.Name[len(prefix):]
		m[fmt.Sprintf("deviation.%s", name)] = f.Value.String()
	})
}

// local populates those test properties that can be
// collected locally without using the testbed reservation.
func local(m map[string]string) {
	buildInfo(m)
	wd := gitInfo(m)
	if tp := testPath(wd); tp != "" {
		m["test.path"] = tp
	}
	deviationInfo(m, flag.CommandLine)
}
