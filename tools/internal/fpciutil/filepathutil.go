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

// Package fpciutil contains filepath related utilities for featureprofiles CI.
package fpciutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// READMEname is the name of all test READMEs according to the contribution guide.
	READMEname = "README.md"
)

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FeatureDir finds the path to the feature directory from CWD.
func FeatureDir() (string, error) {
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("could not detect caller")
	}
	newpath := filepath.Dir(path)
	for newpath != "." && newpath != "/" {
		featurepath := filepath.Join(newpath, "feature")
		if isDir(featurepath) {
			return featurepath, nil
		}
		newpath = filepath.Dir(newpath)
	}
	return "", fmt.Errorf("feature root not found from %s", path)
}
