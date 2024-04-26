// Copyright 2024 Google LLC
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

// Command validate_readme_spec validates Paths and RPCs listed by MarkDown
// (READMEs) against the most recent repository states in
// github.com/openconfig.
//
// Note: For `rpcs`, only the RPC name and methods are validated. Any
// attributes defined below RPC methods (e.g. union_replace) are not validated.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/tools/internal/fpciutil"
	"github.com/openconfig/featureprofiles/tools/internal/mdocspec"
	"github.com/openconfig/featureprofiles/tools/internal/ocpaths"
	"github.com/openconfig/featureprofiles/tools/internal/ocrpcs"
	"golang.org/x/exp/maps"
)

// Config is the set of flags for this binary.
type Config struct {
	DownloadPath string
	FeatureDir   string
}

// New registers a flagset with the configuration needed by this binary.
func New(fs *flag.FlagSet) *Config {
	c := &Config{}

	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&c.DownloadPath, "download-path", "./tmp", "path into which to download OpenConfig GitHub repos for validation")
	fs.StringVar(&c.FeatureDir, "feature-dir", "", "path to the feature directory of featureprofiles, for which all README.md files are validated for their coverage spec aside from the allow-list in readme_allowlist.go")

	return c
}

var (
	config *Config
)

func init() {
	config = New(nil)
}

func readmeFiles(featureDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(featureDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() != fpciutil.READMEname {
			return nil
		}
		relpath, err := filepath.Rel(filepath.Dir(featureDir), path)
		if err != nil {
			return fmt.Errorf("unexpected error: cannot take relative path of file %q against feature directory %q", path, featureDir)
		}
		if _, ok := nonTestREADMEs[relpath]; ok {
			// Allowlist
			return nil
		}

		files = append(files, path)
		return nil
	})
	return files, err
}

func main() {
	flag.Parse()

	fileCount := flag.NArg()
	var files []string
	switch {
	case fileCount != 0 && config.FeatureDir != "":
		log.Exit("If -feature-dir flag is specified, README files must not be specified as positional arguments.")
	case fileCount == 0 && config.FeatureDir == "":
		var err error
		config.FeatureDir, err = fpciutil.FeatureDir()
		if err != nil {
			log.Exitf("Unable to locate feature root: %v", err)
		}
		fallthrough
	case config.FeatureDir != "":
		var err error
		files, err = readmeFiles(config.FeatureDir)
		if err != nil {
			log.Exitf("Error gathering README.md files for validation: %v", err)
		}
	case fileCount != 0:
		files = flag.Args()
	default:
		log.Exit("Program internal error: input not handled.")
	}

	if err := os.MkdirAll(config.DownloadPath, 0750); err != nil {
		fmt.Println(fmt.Errorf("cannot create download path directory: %v", config.DownloadPath))
	}
	publicPath, err := ocpaths.ClonePublicRepo(config.DownloadPath, "")
	if err != nil {
		log.Exit(err)
	}

	erredFiles := map[string]struct{}{}
	for _, file := range files {
		log.Infof("Validating %q", file)
		b, err := os.ReadFile(file)
		if err != nil {
			log.Exitf("Error reading file: %q", file)
		}
		ocPaths, ocRPCs, err := mdocspec.Parse(b)
		if err != nil {
			log.Errorf("file %v: %v", file, err)
			erredFiles[file] = struct{}{}
			continue
		}

		paths, invalidPaths, err := ocpaths.ValidatePaths(ocPaths.GetOcpaths(), publicPath)
		if err != nil {
			log.Errorf("%q contains %d invalid OCPaths:\n%v", file, len(invalidPaths), err)
			erredFiles[file] = struct{}{}
		} else {
			log.Infof("%q contains %d valid OCPaths\n", file, len(paths))
		}

		rpcValidCount, err := ocrpcs.ValidateRPCs(config.DownloadPath, ocRPCs.GetOcProtocols())
		if err != nil {
			log.Errorf("%q contains invalid RPCs: %v", file, err)
			erredFiles[file] = struct{}{}
		} else {
			log.Infof("%q contains %d valid OCRPCs\n", file, rpcValidCount)
		}
	}
	if len(erredFiles) > 0 {
		log.Exitf("The following files have errors:\n%v", strings.Join(maps.Keys(erredFiles), "\n"))
	}
}
