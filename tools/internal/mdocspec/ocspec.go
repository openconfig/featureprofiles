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

// Package mdocspec parses yaml OC requirements from functional test READMEs.
package mdocspec

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/yuin/goldmark"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
)

// ErrNotFound indicates the OpenConfig Path and RPC Coverage YAML block was
// not found or was invalid.
var ErrNotFound = fmt.Errorf(`did not detect valid yaml block under a heading titled %q, please see https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md for example`, OCSpecHeading)

// Parse extracts sorted OpenConfig Path and RPC Coverage from a
// featureprofiles README.
//
// If such a coverage section is not found in the README, `ErrNotFound` will be
// returned.
//
// Expected markdown format:
//
//	## OpenConfig Path and RPC Coverage
//
//	```yaml
//	paths:
//	  /interfaces/interface/config/description:
//	  /interfaces/interface/config/enabled:
//	  /components/component/state/name:
//	    platform_type: [
//	      "CHASSIS"
//	      "CONTROLLER_CARD",
//	      "LINECARD",
//	      "FABRIC",
//	    ]
//
//	rpcs:
//	  gnmi:
//	    gNMI.Set:
//	      union_replace: true
//	    gNMI.Subscribe:
//	      on_change: true
//	```
//
// The first yaml code block after a heading line named exactly as
// "OpenConfig Path and RPC Coverage" will be parsed. Any other code blocks are
// ignored.
func Parse(source []byte) (*ppb.OCPaths, *rpb.OCRPCs, error) {
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(MDOCSpecs),
	)
	if err := md.Convert(source, &buf); err != nil {
		return nil, nil, fmt.Errorf("MDOCSpec.Convert: %v", err)
	}
	if buf.Len() == 0 {
		return nil, nil, ErrNotFound
	}

	return parseYAML(buf.Bytes())
}

func parseYAML(source []byte) (*ppb.OCPaths, *rpb.OCRPCs, error) {
	s := map[string]map[string]map[string]any{}
	if err := yaml.Unmarshal(source, &s); err != nil {
		return nil, nil, fmt.Errorf("mdocspec: error parsing YAML: %v", err)
	}

	protoPaths := &ppb.OCPaths{}

	paths := s["paths"]
	pathNames := maps.Keys(paths)
	sort.Strings(pathNames)
	for _, name := range pathNames {
		platformTypes := map[string]struct{}{}
		for propertyName, property := range paths[name] {
			switch propertyName {
			case "platform_type":
				ps, ok := property.([]any)
				if !ok {
					return nil, nil, fmt.Errorf("mdocspec: path %q: got (%T, %v) for `platform_type` attribute, but expected []any", name, property, property)
				}
				if len(ps) == 0 {
					return nil, nil, fmt.Errorf("mdocspec: path %q: `platform_type` attribute must not be empty", name)
				}
				for i, p := range ps {
					sp, ok := p.(string)
					if !ok {
						return nil, nil, fmt.Errorf("mdocspec: path %q: got (%T, %v), for `platform_type` element index %v, but must be string", name, p, p, i)
					}
					if _, ok := platformTypes[sp]; ok {
						return nil, nil, fmt.Errorf("mdocspec: path %q: got duplicate element %q for `platform_type` element index %v", name, sp, i)
					}
					platformTypes[sp] = struct{}{}
				}
			case "value", "values": // Accept value/values as a property names used to specify what property of the path is used in the test.
			default:
				return nil, nil, fmt.Errorf("mdocspec: path %q: only `platform_type` is expected as a valid attribute for paths, got %q", name, propertyName)
			}
		}
		if len(platformTypes) == 0 {
			protoPaths.Ocpaths = append(protoPaths.Ocpaths, &ppb.OCPath{
				Name: name,
			})
			continue
		}
		platformTypesSlice := maps.Keys(platformTypes)
		sort.Strings(platformTypesSlice)
		for _, platformType := range platformTypesSlice {
			protoPaths.Ocpaths = append(protoPaths.Ocpaths, &ppb.OCPath{
				Name: name,
				OcpathConstraint: &ppb.OCPathConstraint{
					Constraint: &ppb.OCPathConstraint_PlatformType{
						PlatformType: platformType,
					},
				},
			})
		}
	}

	protoRPCs := &rpb.OCRPCs{
		OcProtocols: map[string]*rpb.OCProtocol{},
	}

	rpcs, ok := s["rpcs"]
	if !ok {
		return nil, nil, fmt.Errorf("mdocspec: YAML does not have mandatory top-level \"rpcs\" attribute")
	}
	rpcNames := maps.Keys(rpcs)
	sort.Strings(rpcNames)
	var hasMethod bool
	for _, name := range rpcNames {
		methods := maps.Keys(rpcs[name])
		if len(methods) > 0 {
			hasMethod = true
		}
		sort.Strings(methods)
		for i, method := range methods {
			methods[i] = name + "." + method
		}
		protoRPCs.OcProtocols[name] = &rpb.OCProtocol{
			MethodName: methods,
		}
	}
	if !hasMethod {
		return nil, nil, fmt.Errorf("mdocspec: YAML does not have least one RPC method specified")
	}

	return protoPaths, protoRPCs, nil
}
