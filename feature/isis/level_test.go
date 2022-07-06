/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package isis

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestLevelAugmentGlobal tests the level augment to ISIS global OC.
func TestLevelAugmentGlobal(t *testing.T) {
	tests := []struct {
		desc     string
		level    *Level
		inISIS   *fpoc.NetworkInstance_Protocol_Isis
		wantISIS *fpoc.NetworkInstance_Protocol_Isis
	}{{
		desc:   "ISIS level with no params",
		level:  NewLevel(2),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
				},
			},
		},
	}, {
		desc:  "Augment with no conflicts",
		level: NewLevel(2),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
				},
			},
		},
		wantISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.level.AugmentGlobal(test.inISIS); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantISIS, test.inISIS); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestLevelAugmentGlobalErrors tests the level augment to ISIS global
// OC validation.
func TestLevelAugmentGlobalErrors(t *testing.T) {
	tests := []struct {
		desc          string
		level         *Level
		inISIS        *fpoc.NetworkInstance_Protocol_Isis
		wantErrSubStr string
	}{{
		desc:  "Augment with conflicts",
		level: NewLevel(2),
		inISIS: &fpoc.NetworkInstance_Protocol_Isis{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(false),
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.level.AugmentGlobal(test.inISIS)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings don't match: %v", err)
			}
		})
	}
}
