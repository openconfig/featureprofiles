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
	"os"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/tools/internal/ocpaths"
	"github.com/openconfig/featureprofiles/tools/internal/ocrpcs"
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
	fs.StringVar(&c.DownloadPath, "download-path", "./tmp", "path into which to download OpenConfig GitHub repos for validation")

	return c
}

var (
	config *Config
)

func init() {
	config = New(nil)
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

	if profile.GetSoftwareVersion() == "" {
		log.Exitln("Software version must be specified")
	}

	if profile.GetHardwareName() == "" {
		log.Exitln("HW name must be specified")
	}

	if err := os.MkdirAll(config.DownloadPath, 0750); err != nil {
		fmt.Println(fmt.Errorf("cannot create download path directory: %v", config.DownloadPath))
	}
	publicPath, err := ocpaths.ClonePublicRepo(config.DownloadPath, "v"+profile.Ocpaths.GetVersion())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var hasErr bool
	paths, invalidPaths, err := ocpaths.ValidatePaths(profile.GetOcpaths().GetOcpaths(), publicPath)
	if err != nil {
		fmt.Printf("profile contains %d invalid OCPaths:\n%v", len(invalidPaths), err)
		fmt.Println(err)
		hasErr = true
	} else {
		fmt.Printf("profile contains %d valid OCPaths\n", len(paths))
	}

	rpcValidCount, err := ocrpcs.ValidateRPCs(config.DownloadPath, profile.GetOcrpcs().GetOcProtocols())
	if err != nil {
		fmt.Println(err)
		hasErr = true
	} else {
		fmt.Printf("profile contains %d valid OCRPCs\n", rpcValidCount)
	}

	if hasErr {
		os.Exit(1)
	}
}
