// Copyright 2025 Google LLC
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

// Package canonicalocspec parses Canonical OCs from featureprofiles READMEs.
package canonicalocspec

import (
	"bytes"
	"fmt"

	"github.com/openconfig/featureprofiles/tools/internal/mdocspec"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"github.com/yuin/goldmark"
)

// ErrNotFound indicates the Canonical OC JSON block was not found or was invalid.
var ErrNotFound = fmt.Errorf("did not detect valid json block under a heading titled %q, please see https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md#canonical-oc for example", mdocspec.CanonicalOCHeading)

// Parse extracts all the Canonical OCs from a featureprofiles README.
// If such a section is not found in the README, `ErrNotFound` will be
// returned.
//
// Expected markdown format:
//
//	## Canonical OC
//
//	```json
//	{
//	  "interfaces": {
//	    "interface": [
//	      {
//	        "config": {
//	          "description": "a description",
//	          "mtu": 1500,
//	          "name": "eth0",
//	          "type": "ethernetCsmacd"
//	        },
//	        "hold-time": {
//	          "config": {
//	            "up": 42
//	          }
//	        },
//	        "name": "eth0"
//	      }
//	    ]
//	  },
//	  "system": {
//	    "config": {
//	      "hostname": "a hostname"
//	    }
//	  }
//	}
//	```
//
// The first JSON code block after a heading line named exactly as
// "Canonical OC" will be parsed. Any other code blocks are
// ignored.
func Parse(source []byte) ([]ygot.GoStruct, error) {
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(mdocspec.MDJSONSpecs),
	)
	if err := md.Convert(source, &buf); err != nil {
		return nil, fmt.Errorf("MDJSONSpecs.Convert: %v", err)
	}
	if len(mdocspec.MDJSONSpecs.CanonicalOCs) == 0 {
		return nil, ErrNotFound
	}
	var ocs []ygot.GoStruct
	for _, oc := range mdocspec.MDJSONSpecs.CanonicalOCs {
		ocStruct, err := getCanonicalOC([]byte(oc))
		if err != nil {
			return nil, err
		}
		ocs = append(ocs, ocStruct)
	}
	return ocs, nil
}

func getCanonicalOC(source []byte) (ygot.GoStruct, error) {
	d := &oc.Root{}
	if err := oc.Unmarshal(source, d, &ytypes.PreferShadowPath{}); err != nil {
		return nil, err
	}
	return d, nil
}
