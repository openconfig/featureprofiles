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

// Package rundata collects the runtime data from the test environment.
//
// The values collected are:
//
//   - build.go_version - from runtime/debug.BuildInfo.GoVersion
//   - build.path - from runtime/debug.BuildInfo.Path
//   - build.main.path - from runtime/debug.BuildInfo.Main.Path
//   - build.main.version - from runtime/debug.BuildInfo.Main.Version
//   - build.main.sum - from runtime/debug.BuildInfo.Main.Sum
//   - For each build setting obtained from runtime/debug.BuildInfo.Settings:
//     build.settings.key - the key and the value from runtime/debug.BuildSetting.
//     Note: vcs details are missing when run from a git local working directory.
//     This is why we include the git properties below.
//   - git.commit - git commit hash of the Feature Profiles repo, shown by git show -s --format=%H.
//   - git.commit_timestamp - git commit timestamp of the Feature Profiles repo, shown by
//     git show -s --format=%ct (in Unix epoch seconds).
//   - git.origin - the output of git config --get remote.origin.url which should be either:
//     https://github.com/openconfig/featureprofiles
//     git@github.com:openconfig/featureprofiles.git
//     Please talk to us if you need to run tests from a fork, e.g. with local modifications that
//     have not being merged to our main repo.
//   - git.clean - true if the current working directory is clean
//     (i.e. the output of git status --short is empty), or false otherwise.
//   - git.status - the output of git status --short which should be empty
//     if the working directory is clean.
//   - test.path - the package path of the test, relative to the git local working directory.
//   - test.plan_id - test plan ID that is optionally reported by the test. See below.
//   - topology - a summary of the testbed topology formatted as a comma separated list of devices in the topology and the number of ports they provide, ordered by the device ID, e.g.
//     "ate:12,dut:12" - represents the atedut_12.testbed,
//     "dut1:4,dut2:4" - represents the dutdut.testbed.
//     The testbed summary is discoverable using the binding reservation,
//     whereas the testbed filename is not.
//   - dut.vendor - the vendor of the DUT.
//   - dut.model - the vendor model name of the DUT.
//   - dut.os_version - the OS version running on the DUT.
package rundata

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/ondatra/binding"
)

var (
	knownIssueURL = flag.String("known_issue_url", "", "Report a known issue that explains why the test fails.  This should be a URL to the issue tracker.")

	// flags to disable collecting dut info.
	collectDUTInfo = flag.Bool("collect_dut_info", true, "This flag specifies if the dut information to be collected before running tests.")

	// Stub out for unit tests.
	metadataGetFn = metadata.Get
)

// topology summarizes the topology from the reservation.
func topology(resv *binding.Reservation) string {
	// top maps from the ATE/DUT device names to the number of ports.
	top := make(map[string]int)

	var topKeys []string
	for ateName, ate := range resv.ATEs {
		top[ateName] = len(ate.Ports())
		topKeys = append(topKeys, ateName)
	}
	for dutName, dut := range resv.DUTs {
		top[dutName] = len(dut.Ports())
		topKeys = append(topKeys, dutName)
	}

	sort.Strings(topKeys)
	var parts []string
	for _, k := range topKeys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, top[k]))
	}
	return strings.Join(parts, ",")
}

// Properties builds the test properties map representing run data.
func Properties(ctx context.Context, resv *binding.Reservation) map[string]string {
	md := metadataGetFn()

	m := make(map[string]string)
	local(m)

	if uuid := md.GetUuid(); uuid != "" {
		m["test.uuid"] = uuid
	}
	if planID := md.GetPlanId(); planID != "" {
		m["test.plan_id"] = planID
	}
	if desc := md.GetDescription(); desc != "" {
		m["test.description"] = desc
	}

	if *knownIssueURL != "" {
		m["known_issue_url"] = *knownIssueURL
	}

	if resv != nil {
		m["topology"] = topology(resv)
		if *collectDUTInfo {
			dutsInfo(ctx, m, resv)
		}
	}

	return m
}

var timeBegin = time.Now()

// Timing builds the test properties with the begin and end times.
func Timing(context.Context) map[string]string {
	m := make(map[string]string)
	m["time.begin"] = fmt.Sprint(timeBegin.Unix())
	m["time.end"] = fmt.Sprint(time.Now().Unix())
	return m
}
