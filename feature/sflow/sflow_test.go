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

// Package sflow implements the Config Library for SFLOW feature profile.
package sflow

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentDevice tests the features of SFLOW config.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc       string
		sflow      *Sflow
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "Sflow globally enabled",
		sflow:    New(),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled: ygot.Bool(true),
				},
			},
		},
	}, {
		desc:     "Sflow with agent-id-ipv4",
		sflow:    New().WithAgentIDIPv4("192.0.2.1"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:     ygot.Bool(true),
					AgentIdIpv4: ygot.String("192.0.2.1"),
				},
			},
		},
	}, {
		desc:     "Sflow with agent-id-ipv6",
		sflow:    New().WithAgentIDIPv6("2001:DB8:0::1"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:     ygot.Bool(true),
					AgentIdIpv6: ygot.String("2001:DB8:0::1"),
				},
			},
		},
	}, {
		desc:     "Sflow with egress-sampling-rate",
		sflow:    New().WithEgressSamplingRate(10),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:            ygot.Bool(true),
					EgressSamplingRate: ygot.Uint32(10),
				},
			},
		},
	}, {
		desc:     "Sflow with ingress-sampling-rate",
		sflow:    New().WithIngressSamplingRate(10),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:             ygot.Bool(true),
					IngressSamplingRate: ygot.Uint32(10),
				},
			},
		},
	}, {
		desc:     "Sflow with sample-size",
		sflow:    New().WithSampleSize(10),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:    ygot.Bool(true),
					SampleSize: ygot.Uint16(10),
				},
			},
		},
	}, {
		desc:  "Device contains Sflow config, no conflicts",
		sflow: New().WithSampleSize(10),
		inDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:    ygot.Bool(true),
					SampleSize: ygot.Uint16(10),
				},
			},
		},
		wantDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					Enabled:    ygot.Bool(true),
					SampleSize: ygot.Uint16(10),
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.sflow.AugmentDevice(test.inDevice); err != nil {
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
		sflow         *Sflow
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc:  "Device contains Sflow config with conflict",
		sflow: New().WithSampleSize(10),
		inDevice: &fpoc.Device{
			Sampling: &fpoc.Sampling{
				Sflow: &fpoc.Sampling_Sflow{
					SampleSize: ygot.Uint16(11),
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.sflow.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}
