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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentInterfaceLevel tests the level augment to ISIS interface OC.
func TestAugmentInterfaceLevel(t *testing.T) {
	tests := []struct {
		desc     string
		level    *InterfaceLevel
		inIntf   *fpoc.NetworkInstance_Protocol_Isis_Interface
		wantIntf *fpoc.NetworkInstance_Protocol_Isis_Interface
	}{{
		desc:   "ISIS interface level with no params",
		level:  NewInterfaceLevel(2),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{},
		wantIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
				},
			},
		},
	}, {
		desc:   "ISIS interface level with hello interval",
		level:  NewInterfaceLevel(2).WithHelloInterval(10 * time.Second),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{},
		wantIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Timers{
						HelloInterval: ygot.Uint32(10),
					},
				},
			},
		},
	}, {
		desc:   "ISIS interface level with hello multiplier",
		level:  NewInterfaceLevel(2).WithHelloMultiplier(10),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{},
		wantIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Timers: &fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Timers{
						HelloMultiplier: ygot.Uint8(10),
					},
				},
			},
		},
	}, {
		desc:   "ISIS interface level with AFI-SAFI metric",
		level:  NewInterfaceLevel(2).WithAFISAFIMetric(fpoc.IsisTypes_AFI_TYPE_IPV4, fpoc.IsisTypes_SAFI_TYPE_UNICAST, 10),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{},
		wantIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Af: map[fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
						{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
							AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
							SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
							Enabled:  ygot.Bool(true),
							Metric:   ygot.Uint32(10),
						},
					},
				},
			},
		},
	}, {
		desc:  "Augment with no conflicts",
		level: NewInterfaceLevel(2).WithAFISAFIMetric(fpoc.IsisTypes_AFI_TYPE_IPV4, fpoc.IsisTypes_SAFI_TYPE_UNICAST, 10),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Af: map[fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
						{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
							AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
							SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
							Enabled:  ygot.Bool(true),
							Metric:   ygot.Uint32(10),
						},
					},
				},
			},
		},
		wantIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Af: map[fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
						{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
							AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
							SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
							Enabled:  ygot.Bool(true),
							Metric:   ygot.Uint32(10),
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.level.AugmentInterface(test.inIntf); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantIntf, test.inIntf); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentInterfaceLevelErrors tests the level augment to ISIS Interface
// OC validation.
func TestAugmentInterfaceLevelErrors(t *testing.T) {
	tests := []struct {
		desc          string
		level         *InterfaceLevel
		inIntf        *fpoc.NetworkInstance_Protocol_Isis_Interface
		wantErrSubStr string
	}{{
		desc:  "Augment with conflicts",
		level: NewInterfaceLevel(2).WithAFISAFIMetric(fpoc.IsisTypes_AFI_TYPE_IPV4, fpoc.IsisTypes_SAFI_TYPE_UNICAST, 11),
		inIntf: &fpoc.NetworkInstance_Protocol_Isis_Interface{
			Level: map[uint8]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
				2: {
					LevelNumber: ygot.Uint8(2),
					Enabled:     ygot.Bool(true),
					Af: map[fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af_Key]*fpoc.NetworkInstance_Protocol_Isis_Interface_Level_Af{
						{AfiName: fpoc.IsisTypes_AFI_TYPE_IPV4, SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST}: {
							AfiName:  fpoc.IsisTypes_AFI_TYPE_IPV4,
							SafiName: fpoc.IsisTypes_SAFI_TYPE_UNICAST,
							Enabled:  ygot.Bool(true),
							Metric:   ygot.Uint32(10),
						},
					},
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.level.AugmentInterface(test.inIntf)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings don't match: %v", err)
			}
		})
	}
}
