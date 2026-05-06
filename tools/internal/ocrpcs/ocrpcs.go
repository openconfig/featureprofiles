// Copyright © 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ocrpcs contains utilities related to ocrpcs.proto.
package ocrpcs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yoheimuta/go-protoparser/v4"

	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
	"github.com/openconfig/gnmi/errlist"
)

// cloneAPIRepo clones the openconfig/<api> repo at the given path.
//
// # Note
//
// * If the folder already exists, then no additional downloads will be made.
// * A manual deletion of the folder is required if no longer used.
func cloneAPIRepo(downloadPath, api string) (string, error) {
	if downloadPath == "" {
		return "", fmt.Errorf("must provide download path")
	}
	repoPath := filepath.Join(downloadPath, api)

	if _, err := os.Stat(repoPath); err == nil { // If NO error
		return repoPath, nil
	}

	cmd := exec.Command("git", "clone", "--single-branch", "--depth", "1", fmt.Sprintf("https://github.com/openconfig/%s.git", api), repoPath)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to clone %s repo: %v, command failed to start: %q", api, err, cmd.String())
	}
	stderrOutput, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to clone %s repo: %v, command failed during execution: %q\n%s", api, err, cmd.String(), stderrOutput)
	}
	return repoPath, nil
}

// Read returns all RPCs for the given OpenConfig API.
//
//   - downloadPath specifies the folder to download the associated OpenConfig
//     repo in order to allow for proto file parsing.
func Read(downloadPath, api string) (map[string]struct{}, error) {
	repoPath, err := cloneAPIRepo(downloadPath, api)
	if err != nil {
		return nil, err
	}

	rpcs := map[string]struct{}{}

	if err := filepath.Walk(repoPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(info.Name(), ".proto") {
				reader, err := os.Open(path)
				if err != nil {
					return err
				}
				got, err := protoparser.Parse(reader)
				if err != nil {
					return fmt.Errorf("failed to parse, err %v", err)
				}
				visitor := &rpcServiceAccumulator{}
				got.Accept(visitor)
				for _, rpc := range visitor.rpcs {
					rpcs[rpc] = struct{}{}
				}
			}
			return nil
		}); err != nil {
		return nil, err
	}

	return rpcs, nil
}

// ValidateRPCs verifies whether the RPCs listed in protocols are valid.
//
// It returns the number of valid RPCs found, and an error if any RPC is
// invalid, or there was an issue downloading or parsing OpenConfig protobuf
// files.
//
// - downloadPath is a path to the folder which will contain OpenConfig
// repositories that will be downloaded in order to validate the existence of
// provided RPCs.
func ValidateRPCs(downloadPath string, protocols map[string]*rpb.OCProtocol) (uint, error) {
	var validCount uint

	var errs errlist.List
	errs.Separator = "\n"
	for api, protocol := range protocols {
		rpcs, err := Read(downloadPath, api)
		if err != nil {
			return 0, err
		}
		for _, name := range protocol.GetMethodName() {
			if _, ok := rpcs[name]; !ok {
				errs.Add(fmt.Errorf("RPC not found in openconfig repo %v: %v", api, name))
				continue
			}
			validCount++
		}
	}

	return validCount, errs.Err()
}
