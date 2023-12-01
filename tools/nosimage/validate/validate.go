// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main validates textprotos of the format specified by nosimage.proto.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/openconfig/featureprofiles/tools/internal/ocpaths"
	"google.golang.org/protobuf/encoding/prototext"

	npb "github.com/openconfig/featureprofiles/proto/nosimage_go_proto"
)

// Config is the set of flags for this binary.
type Config struct {
	FilePath     string
	DownloadPath string
}

// New registers a flagset with the configuration needed by this binary.
func New(fs *flag.FlagSet) *Config {
	c := &Config{}

	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&c.FilePath, "file", "", "txtpb file containing an instance of nosimage.proto data")
	// TODO(wenovus): Consider allowing using a manual location to avoid git clone.
	fs.StringVar(&c.DownloadPath, "download-path", "./", "path into which to download OpenConfig GitHub repos for validation")

	return c
}

var (
	config *Config
)

func init() {
	config = New(nil)
}

func clonePublicRepo(downloadPath, branch string) (string, error) {
	if downloadPath == "" {
		return "", fmt.Errorf("must provide download path")
	}
	publicPath := filepath.Join(config.DownloadPath, "public")

	cmd := exec.Command("git", "clone", "-b", branch, "--single-branch", "--depth", "1", "git@github.com:openconfig/public.git", publicPath)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to clone public repo: %v, command failed to start: %q", err, cmd.String())
	}
	stderrOutput, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to clone public repo: %v, command failed during execution: %q\n%s", err, cmd.String(), stderrOutput)
	}
	return publicPath, nil
}

func unmarshalFile(filePath string) (*npb.NOSImageProfile, error) {
	if filePath == "" {
		return nil, fmt.Errorf("must provide non-empty file path to read from")
	}
	profile := &npb.NOSImageProfile{}
	bs, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if err := prototext.Unmarshal(bs, profile); err != nil {
		return nil, err
	}
	return profile, nil

}

func main() {
	flag.Parse()

	profile, err := unmarshalFile(config.FilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	publicPath, err := clonePublicRepo(config.DownloadPath, "v"+profile.Ocpaths.GetVersion())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	paths, err := ocpaths.ValidatePaths(profile.GetOcpaths().GetOcpaths(), publicPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("profile contains %d valid OCPaths\n", len(paths))
}
