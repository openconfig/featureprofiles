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

package sflow

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

var key = fpoc.Sampling_Sflow_Collector_Key{Address: "192.0.2.1", Port: 9000}

// TestCollectorAugmentSflow tests the collector augment to Sflow.
func TestCollectorAugmentSflow(t *testing.T) {
	tests := []struct {
		desc      string
		collector *Collector
		inSflow   *fpoc.Sampling_Sflow
		wantSflow *fpoc.Sampling_Sflow
	}{{
		desc:      "New collector",
		collector: NewCollector("192.0.2.1", 9000),
		inSflow:   &fpoc.Sampling_Sflow{},
		wantSflow: &fpoc.Sampling_Sflow{
			Collector: map[fpoc.Sampling_Sflow_Collector_Key]*fpoc.Sampling_Sflow_Collector{
				key: {
					Address: ygot.String("192.0.2.1"),
					Port:    ygot.Uint16(9000),
				},
			},
		},
	}, {
		desc:      "Collector with network-instance",
		collector: NewCollector("192.0.2.1", 9000).WithNetworkInstance("vrf-1"),
		inSflow:   &fpoc.Sampling_Sflow{},
		wantSflow: &fpoc.Sampling_Sflow{
			Collector: map[fpoc.Sampling_Sflow_Collector_Key]*fpoc.Sampling_Sflow_Collector{
				key: {
					Address:         ygot.String("192.0.2.1"),
					Port:            ygot.Uint16(9000),
					NetworkInstance: ygot.String("vrf-1"),
				},
			},
		},
	}, {
		desc:      "Sflow already contains collector with no conflicts",
		collector: NewCollector("192.0.2.1", 9000).WithNetworkInstance("vrf-1"),
		inSflow: &fpoc.Sampling_Sflow{
			Collector: map[fpoc.Sampling_Sflow_Collector_Key]*fpoc.Sampling_Sflow_Collector{
				key: {
					Address:         ygot.String("192.0.2.1"),
					Port:            ygot.Uint16(9000),
					NetworkInstance: ygot.String("vrf-1"),
				},
			},
		},
		wantSflow: &fpoc.Sampling_Sflow{
			Collector: map[fpoc.Sampling_Sflow_Collector_Key]*fpoc.Sampling_Sflow_Collector{
				key: {
					Address:         ygot.String("192.0.2.1"),
					Port:            ygot.Uint16(9000),
					NetworkInstance: ygot.String("vrf-1"),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.collector.AugmentSflow(test.inSflow); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantSflow, test.inSflow); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestCollectorAugmentSflowErrors tests the collector augment to Sflow validation.
func TestCollectorAugmentSflowErrors(t *testing.T) {
	tests := []struct {
		desc          string
		collector     *Collector
		inSflow       *fpoc.Sampling_Sflow
		wantErrSubStr string
	}{{
		desc:      "Sflow already contains collector with conflicts",
		collector: NewCollector("192.0.2.1", 9000).WithNetworkInstance("vrf-1"),
		inSflow: &fpoc.Sampling_Sflow{
			Collector: map[fpoc.Sampling_Sflow_Collector_Key]*fpoc.Sampling_Sflow_Collector{
				key: {
					Address:         ygot.String("192.0.2.1"),
					Port:            ygot.Uint16(9000),
					NetworkInstance: ygot.String("vrf-2"),
				},
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.collector.AugmentSflow(test.inSflow)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error string does not match: %v", err)
			}
		})
	}
}
