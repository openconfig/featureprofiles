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

package ocpaths

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/openconfig/gnmi/errlist"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/goyang/pkg/yangentry"
	"github.com/openconfig/models-ci/yangutil"
	"github.com/openconfig/ondatra/gnmi/oc"

	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
)

const (
	componentPrefix = "/components/component"
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
	isComponent := strings.HasPrefix(path, componentPrefix)
	matched, err := regexp.MatchString("^[A-Z_]+$", component)
	if err != nil {
		return nil, err
	}
CONSTRAINT_CHECK:
	switch {
	case !isComponent && component != "":
		return nil, fmt.Errorf("non-component path %q has component value %q", path, component)
	case !isComponent:
		// valid
	case matched:
		for _, enum := range []string{
			reflect.TypeOf(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT(0)).Name(),
			reflect.TypeOf(oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT(0)).Name(),
		} {
			for _, v := range oc.ΛEnum[enum] {
				if v.Name == component {
					ocpath.Key.Component = component
					break CONSTRAINT_CHECK
				}
			}
		}
		fallthrough
	default:
		return nil, fmt.Errorf("path %q has invalid component %q", path, component)
	}

	// Basic validation of featureprofileid if it exists
	featureprofileID := ocpath.FeatureprofileID
	if featureprofileID != "" {
		matched, err := regexp.MatchString("^([a-z0-9]+_)*[a-z0-9]+$", featureprofileID)
		if err != nil {
			return nil, err
		}
		switch {
		case matched:
			ocpath.FeatureprofileID = featureprofileID
		default:
			return nil, fmt.Errorf("unexpected featureprofileID string %q for path %v", featureprofileID, path)
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
