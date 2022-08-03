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

package aggregate

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

const (
	aggID   = "AggregateInterfaceName"
	member1 = "MemberInterface1"
	member2 = "MemberInterface2"
)

// TestAugmentDevice tests the features of Interface config.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc       string
		agg        *Aggregate
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "New aggregate interface",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}, {
		desc:     "Min links",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).WithMinLinks(2),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType:  fpoc.IfAggregate_AggregationType_LACP,
						MinLinks: ygot.Uint16(2),
					},
				},
			},
		},
	}, {
		desc:     "LACP mode",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).WithLACPMode(fpoc.Lacp_LacpActivityType_ACTIVE),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
						LacpMode: fpoc.Lacp_LacpActivityType_ACTIVE,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}, {
		desc:     "System ID MAC",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).WithSystemIDMAC("52:fe:7c:91:6e:c1"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:        ygot.String(aggID),
						Interval:    fpoc.Lacp_LacpPeriodType_FAST,
						SystemIdMac: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}, {
		desc:     "Interface system priority",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).WithInterfaceSystemPriority(2),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:           ygot.String(aggID),
						Interval:       fpoc.Lacp_LacpPeriodType_FAST,
						SystemPriority: ygot.Uint16(2),
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}, {
		desc:     "Global system priority",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).WithGlobalSystemPriority(2),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				SystemPriority: ygot.Uint16(2),
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}, {
		desc:     "Add one member",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).AddMember(member1),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
				member1: {
					Name: ygot.String(member1),
					Ethernet: &fpoc.Interface_Ethernet{
						AggregateId: ygot.String(aggID),
					},
				},
			},
		},
	}, {
		desc:     "Add multiple members",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).AddMember(member1).AddMember(member2),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
				member1: {
					Name: ygot.String(member1),
					Ethernet: &fpoc.Interface_Ethernet{
						AggregateId: ygot.String(aggID),
					},
				},
				member2: {
					Name: ygot.String(member2),
					Ethernet: &fpoc.Interface_Ethernet{
						AggregateId: ygot.String(aggID),
					},
				},
			},
		},
	}, {
		desc:     "Add same members twice",
		agg:      New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST).AddMember(member1).AddMember(member1),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
				member1: {
					Name: ygot.String(member1),
					Ethernet: &fpoc.Interface_Ethernet{
						AggregateId: ygot.String(aggID),
					},
				},
			},
		},
	}, {
		desc: "Device contains Aggregate Interface config, no conflicts",
		agg:  New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST),
		inDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
		wantDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_FAST,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.agg.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentDeviceErrors tests the error handling of AugmentDevice.
func TestAugmentDeviceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		agg           *Aggregate
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc: "Device contains Aggregate Interface config with conflicts",
		agg:  New(aggID, fpoc.IfAggregate_AggregationType_LACP, fpoc.Lacp_LacpPeriodType_FAST),
		inDevice: &fpoc.Device{
			Lacp: &fpoc.Lacp{
				Interface: map[string]*fpoc.Lacp_Interface{
					aggID: {
						Name:     ygot.String(aggID),
						Interval: fpoc.Lacp_LacpPeriodType_SLOW,
					},
				},
			},
			Interface: map[string]*fpoc.Interface{
				aggID: {
					Name: ygot.String(aggID),
					Aggregation: &fpoc.Interface_Aggregation{
						LagType: fpoc.IfAggregate_AggregationType_LACP,
					},
				},
			},
		},
		wantErrSubStr: "destination and source values were set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.agg.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}
