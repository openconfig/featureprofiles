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

package networkinstance

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugment tests the NI augment to device OC.
func TestAugment(t *testing.T) {
	tests := []struct {
		desc       string
		ni         *NetworkInstance
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "Default NI",
		ni:       New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"DEFAULT": {
					Name:    ygot.String("DEFAULT"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
			},
		},
	}, {
		desc:     "L3VRF",
		ni:       New("vrf-1", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"vrf-1": {
					Name:    ygot.String("vrf-1"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
				},
			},
		},
	}, {
		desc: "Device contains another VRF with no conflicts",
		ni:   New("vrf-1", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF),
		inDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"DEFAULT": {
					Name:    ygot.String("DEFAULT"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
			},
		},
		wantDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"DEFAULT": {
					Name:    ygot.String("DEFAULT"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
				"vrf-1": {
					Name:    ygot.String("vrf-1"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
				},
			},
		},
	}, {
		desc: "Device contains same VRF with no conflicts",
		ni:   New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE),
		inDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"DEFAULT": {
					Name:    ygot.String("DEFAULT"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
			},
		},
		wantDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"DEFAULT": {
					Name:    ygot.String("DEFAULT"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.ni.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected: %v", err)
			}

			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentErrors tests validation of NI augment to device OC.
func TestAugmentErrors(t *testing.T) {
	tests := []struct {
		desc          string
		ni            *NetworkInstance
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc:          "Empty NI name",
		ni:            New("", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF),
		inDevice:      &fpoc.Device{},
		wantErrSubStr: "name is empty",
	}, {
		desc:          "NI type is not set",
		ni:            New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_UNSET),
		inDevice:      &fpoc.Device{},
		wantErrSubStr: "type is unset",
	}, {
		desc: "Device contains same VRF with conflicts",
		ni:   New("vrf-1", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF),
		inDevice: &fpoc.Device{
			NetworkInstance: map[string]*fpoc.NetworkInstance{
				"vrf-1": {
					Name:    ygot.String("vrf-1"),
					Enabled: ygot.Bool(true),
					Type:    fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
				},
			},
		},
		wantErrSubStr: "destination and source values were set when merging enum field",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.ni.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings are not equal: %v", err)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	ni            *fpoc.NetworkInstance
}

func (f *FakeFeature) AugmentNetworkInstance(ni *fpoc.NetworkInstance) error {
	f.ni = ni
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
	tests := []struct {
		desc          string
		wantErrSubStr error
	}{{
		desc: "error not expected",
	}, {
		desc:          "error expected",
		wantErrSubStr: errors.New("some error"),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ni := New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
			ff := &FakeFeature{Err: test.wantErrSubStr}
			gotErr := ni.WithFeature(ff)
			if !ff.augmentCalled {
				t.Errorf("AugmentNetworkInstance was not called")
			}
			if ff.ni != &ni.oc {
				t.Errorf("NI ptr is not equal")
			}
			if test.wantErrSubStr != nil {
				if gotErr != nil {
					if !strings.Contains(gotErr.Error(), test.wantErrSubStr.Error()) {
						t.Errorf("Error strings are not equal")
					}
				}
				if gotErr == nil {
					t.Errorf("Expecting error but got none")
				}
			}
		})
	}
}
