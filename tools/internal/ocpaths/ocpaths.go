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

// Package ocpaths contains utilities and types for validating a set of OCPaths
// specified by ocpaths.proto.
package ocpaths

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/errlist"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/goyang/pkg/yangentry"
	"github.com/openconfig/models-ci/yangutil"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/exp/maps"

	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
)

const (
	componentPrefix       = "/components/component"
	featureprofileIDRegex = "^([a-z0-9]+_)*[a-z0-9]+$"
)

var (
	featureprofileIDMatcher = regexp.MustCompile(featureprofileIDRegex)
	validComponentNames     = func() map[string]struct{} {
		names := map[string]struct{}{}
		for _, enum := range []string{
			reflect.TypeOf(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT(0)).Name(),
			reflect.TypeOf(oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT(0)).Name(),
		} {
			for _, v := range oc.ΛEnum[enum] {
				names[v.Name] = struct{}{}
			}
		}
		return names
	}()
	validComponentNamesSorted = func() []string {
		compNames := maps.Keys(validComponentNames)
		sort.Strings(compNames)
		return compNames
	}()
)

// OCPathKey contains the fields that uniquely identify an OC path.
type OCPathKey struct {
	Path      string
	Component string
}

// OCPath is the parsed version of the spreadsheet's paths.
type OCPath struct {
	Key              OCPathKey
	FeatureprofileID string
}

func getSchemaFakeroot(publicPath string) (*yang.Entry, error) {
	files, err := yangutil.GetAllYANGFiles(publicPath)
	if err != nil {
		return nil, err
	}

	moduleEntryMap, errs := yangentry.Parse(files, []string{publicPath})
	if errs != nil {
		return nil, err
	}
	root := &yang.Entry{
		Dir: map[string]*yang.Entry{},
	}
	for _, entry := range moduleEntryMap {
		// Skip IETF modules.
		if !strings.HasPrefix(entry.Name, "openconfig-") {
			continue
		}
		for name, ch := range entry.Dir {
			root.Dir[name] = ch
		}
	}

	return root, nil
}

func validatePath(ocpathProto *ppb.OCPath, root *yang.Entry) (*OCPath, error) {
	ocpath := &OCPath{
		Key: OCPathKey{
			Path:      ocpathProto.GetName(),
			Component: ocpathProto.GetOcpathConstraint().GetPlatformType(),
		},
		FeatureprofileID: ocpathProto.GetFeatureprofileid(),
	}

	// Validate path
	path := ocpath.Key.Path
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path does not begin with slash: %q", path)
	}
	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("path must not end with slash: %q", path)
	}
	if entry := root.Find(path); entry == nil {
		deepestEntry := root
		var entryNotFound string
		splitEles := strings.Split(path, "/")
		for i, ele := range splitEles {
			// Skip the first element, which must be empty.
			if i == 0 {
				continue
			}
			next := deepestEntry.Dir[ele]
			if next == nil {
				entryNotFound = strings.Join(splitEles[i:], "/")
				break
			}
			deepestEntry = next
		}
		return nil, fmt.Errorf("path not found: %q, remaining path: %q", path, entryNotFound)
	} else if !entry.IsLeaf() && !entry.IsLeafList() {
		return nil, fmt.Errorf("path %q is not a leaf: got kind %s", path, entry.Kind)
	}

	// Validate component
	component := ocpath.Key.Component
	isComponentPath := strings.HasPrefix(path, componentPrefix)
	switch {
	case !isComponentPath && component != "":
		return nil, fmt.Errorf("non-component path %q has component value %q", path, component)
	case !isComponentPath:
	default:
		if _, ok := validComponentNames[component]; !ok {
			return nil, fmt.Errorf("path %q has invalid component %q (must be one of %v)", path, component, validComponentNamesSorted)
		}
		ocpath.Key.Component = component
	}

	// featureprofileID is optional. Only validate the string format if it exists.
	if featureprofileID := ocpath.FeatureprofileID; featureprofileID != "" {
		switch {
		case featureprofileIDMatcher.MatchString(featureprofileID):
			ocpath.FeatureprofileID = featureprofileID
		default:
			return nil, fmt.Errorf("unexpected featureprofileID string %q for path %v (must match regex %q)", featureprofileID, path, featureprofileIDRegex)
		}
	}

	return ocpath, nil
}

func insert(dstMap map[OCPathKey]*OCPath, src *OCPath) error {
	if src == nil {
		return fmt.Errorf("provided OCPath is nil")
	}
	if _, ok := dstMap[src.Key]; ok {
		return fmt.Errorf("duplicate entry: %+v", src.Key)
	}
	dstMap[src.Key] = src
	return nil
}

// ValidatePaths parses and validates ocpaths, and puts them into a more
// user-friendly Go structure.
func ValidatePaths(ocpathsProto []*ppb.OCPath, publicPath string) (map[OCPathKey]*OCPath, error) {
	root, err := getSchemaFakeroot(publicPath)
	if err != nil {
		return nil, err
	}

	ocpaths := map[OCPathKey]*OCPath{}
	errs := errlist.List{
		Separator: "\n",
	}
	for _, ocpathProto := range ocpathsProto {
		ocpath, err := validatePath(ocpathProto, root)
		if err != nil {
			errs.Add(err)
		} else if ocpath == nil {
			errs.Add(fmt.Errorf("failed to parse proto: %v", ocpathProto))
		} else if err := insert(ocpaths, ocpath); err != nil {
			errs.Add(err)
		}
	}

	return ocpaths, errs.Err()
}
