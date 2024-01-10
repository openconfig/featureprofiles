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

func cloneAPIRepo(downloadPath, api string) (string, error) {
	if downloadPath == "" {
		return "", fmt.Errorf("must provide download path")
	}
	repoPath := filepath.Join(downloadPath, api)

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

func readAllRPCs(downloadPath, api string) (map[string]struct{}, error) {
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
		rpcs, err := readAllRPCs(downloadPath, api)
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
