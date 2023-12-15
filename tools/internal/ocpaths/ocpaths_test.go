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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
	"github.com/openconfig/goyang/pkg/yang"

	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
)

func getFakeroot(t *testing.T) *yang.Entry {
	root, err := getSchemaFakeroot("testdata/models")
	if err != nil {
		t.Fatal(err)
	}

	return root
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		desc          string
		inOcPathProto *ppb.OCPath
		wantOCPath    *OCPath
		wantErrSubstr string
	}{{
		desc: "no-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name",
			Featureprofileid: "interface_base",
		},
		wantOCPath: &OCPath{
			Key: OCPathKey{
				Path:      "/interfaces/interface/config/name",
				Component: "",
			},
			FeatureprofileID: "interface_base",
		},
	}, {
		desc: "non-leaf path",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `"/interfaces/interface" is not a leaf: got kind Directory`,
	}, {
		desc: "with-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/components/component/state/name",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "CPU"}},
			Featureprofileid: "interface_base",
		},
		wantOCPath: &OCPath{
			Key: OCPathKey{
				Path:      "/components/component/state/name",
				Component: "CPU",
			},
			FeatureprofileID: "interface_base",
		},
	}, {
		desc: "component-path-doesnt-have-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/components/component/state/name",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path "/components/component/state/name" has invalid component "" (must be one of ` + fmt.Sprint(validComponentNamesSorted),
	}, {
		desc: "non-component-path-has-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "CPU"}},
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `non-component path "/interfaces/interface/config/name" has component value "CPU"`,
	}, {
		desc: "invalid-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/components/component/state/name",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "cpu"}},
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path "/components/component/state/name" has invalid component "cpu" (must be one of ` + fmt.Sprint(validComponentNamesSorted),
	}, {
		desc: "with-bad-component",
		inOcPathProto: &ppb.OCPath{
			Name:             "/components/component/state/name",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "NOT-A-COMPONENT"}},
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path "/components/component/state/name" has invalid component "NOT-A-COMPONENT" (must be one of ` + fmt.Sprint(validComponentNamesSorted),
	}, {
		desc: "spaces-after-path",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name   ",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path not found: "/interfaces/interface/config/name   "`,
	}, {
		desc: "extra-slash",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config//name",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path not found: "/interfaces/interface/config//name"`,
	}, {
		desc: "no-starting-slash",
		inOcPathProto: &ppb.OCPath{
			Name:             "interfaces/interface/config/name",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path does not begin with slash: "interfaces/interface/config/name"`,
	}, {
		desc: "ending-slash",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name/",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path must not end with slash: "/interfaces/interface/config/name/"`,
	}, {
		desc: "path-not-found",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/intraface/config/name",
			Featureprofileid: "interface_base",
		},
		wantErrSubstr: `path not found: "/interfaces/interface/intraface/config/name"`,
	}, {
		desc: "invalid-featureprofileid",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name",
			Featureprofileid: "Interface_Base",
		},
		wantErrSubstr: `unexpected featureprofileID string "Interface_Base" for path /interfaces/interface/config/name (must match regex "^([a-z0-9]+_)*[a-z0-9]+$")`,
	}, {
		desc: "invalid-featureprofileid-2",
		inOcPathProto: &ppb.OCPath{
			Name:             "/interfaces/interface/config/name",
			Featureprofileid: "interface-base",
		},
		wantErrSubstr: `unexpected featureprofileID string "interface-base" for path /interfaces/interface/config/name (must match regex "^([a-z0-9]+_)*[a-z0-9]+$")`,
	}}

	root := getFakeroot(t)

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := validatePath(tt.inOcPathProto, root)
			if diff := errdiff.Substring(err, tt.wantErrSubstr); diff != "" {
				t.Errorf(diff)
			}

			if diff := cmp.Diff(tt.wantOCPath, got); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}

func TestValidatePaths(t *testing.T) {
	tests := []struct {
		desc           string
		inOcPathsProto []*ppb.OCPath
		wantOCPaths    map[OCPathKey]*OCPath
		wantErr        bool
	}{{
		desc: "valid",
		inOcPathsProto: []*ppb.OCPath{{
			Name:             "/interfaces/interface/config/name",
			Featureprofileid: "interface_base",
		}, {
			Name:             "/components/component/config/description",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "CPU"}},
			Featureprofileid: "interface_base",
		}, {
			Name:             "/components/component/config/description",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "PORT"}},
			Featureprofileid: "interface_basic",
		}},
		wantOCPaths: map[OCPathKey]*OCPath{
			{
				Path:      "/interfaces/interface/config/name",
				Component: "",
			}: {
				Key: OCPathKey{
					Path: "/interfaces/interface/config/name",
				},
				FeatureprofileID: "interface_base",
			},
			{
				Path:      "/components/component/config/description",
				Component: "CPU",
			}: {
				Key: OCPathKey{
					Path:      "/components/component/config/description",
					Component: "CPU",
				},
				FeatureprofileID: "interface_base",
			},
			{
				Path:      "/components/component/config/description",
				Component: "PORT",
			}: {
				Key: OCPathKey{
					Path:      "/components/component/config/description",
					Component: "PORT",
				},
				FeatureprofileID: "interface_basic",
			},
		},
	}, {
		desc: "invalid-path",
		inOcPathsProto: []*ppb.OCPath{{
			Name:             "/interfaces/interface/config",
			Featureprofileid: "interface_base",
		}},
		wantErr: true,
	}, {
		desc: "duplicate",
		inOcPathsProto: []*ppb.OCPath{{
			Name:             "/interfaces/interface/config/name",
			Featureprofileid: "interface_base",
		}, {
			Name:             "/components/component/config/description",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "CPU"}},
			Featureprofileid: "interface_base",
		}, {
			Name:             "/components/component/config/description",
			OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "CPU"}},
			Featureprofileid: "interface_basic",
		}},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ValidatePaths(tt.inOcPathsProto, "testdata/models")
			if (err != nil) != tt.wantErr {
				t.Fatalf("gotErr: %v, wantErr: %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tt.wantOCPaths, got); diff != "" {
				t.Errorf("(-want, +got):\n%s", diff)
			}
		})
	}
}
