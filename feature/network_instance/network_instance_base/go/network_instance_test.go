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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	tests := []struct {
		desc   string
		name   string
		niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
	}{{
		desc:   "default NI",
		name:   "default",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	}, {
		desc:   "L3VRF",
		name:   "vrf-1",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}}

	for _, test := range tests {
		want := &oc.NetworkInstance{
			Name:    ygot.String(test.name),
			Type:    test.niType,
			Enabled: ygot.Bool(true),
		}
		got := New(test.name, test.niType)
		assert.NotNil(t, got, "New returned nil")
		if diff := cmp.Diff(want, got.oc, protocmp.Transform()); diff != "" {
			t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
		}
	}
}

// TestAugmentDevice tests the NI augment to device OC.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc   string
		name   string
		niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
		device *oc.Device
	}{{
		desc:   "empty device",
		name:   "default",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		device: &oc.Device{},
	}, {
		desc:   "device contains some VRF with no conflict",
		name:   "vrf-1",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		device: func() *oc.Device {
			d := &oc.Device{}
			d.GetOrCreateNetworkInstance("default").Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
			return d
		}(),
	}}

	for _, test := range tests {
		ni := &NetworkInstance{
			oc: &oc.NetworkInstance{
				Name:    ygot.String(test.name),
				Type:    test.niType,
				Enabled: ygot.Bool(true),
			},
		}
		dcopy, err := ygot.DeepCopy(test.device)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		wantDevice := dcopy.(*oc.Device)

		err = ni.AugmentDevice(test.device)
		assert.NoError(t, err, "error not expected")

		if err := wantDevice.AppendNetworkInstance(ni.oc); err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if diff := cmp.Diff(wantDevice, test.device, protocmp.Transform()); diff != "" {
			t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
		}
	}
}

// TestAugmentDeviceErrors tests validation of NI augment to device OC.
func TestAugmentDeviceErrors(t *testing.T) {
	tests := []struct {
		desc   string
		name   string
		niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
		device *oc.Device
		err    string
	}{{
		desc:   "empty NI name",
		name:   "",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		device: &oc.Device{},
		err:    "name is empty",
	}, {
		desc:   "NI type is not set",
		name:   "default",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_UNSET,
		device: &oc.Device{},
		err:    "type is unset",
	}, {
		desc:   "VRF already exists",
		name:   "default",
		niType: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		device: func() *oc.Device {
			d := &oc.Device{}
			d.GetOrCreateNetworkInstance("default").Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
			return d
		}(),
		err: "duplicate key",
	}}

	for _, test := range tests {
		ni := &NetworkInstance{
			oc: &oc.NetworkInstance{
				Name:    ygot.String(test.name),
				Type:    test.niType,
				Enabled: ygot.Bool(true),
			},
		}
		err := ni.AugmentDevice(test.device)
		assert.Error(t, err, "error expected")
		assert.Contains(t, err.Error(), test.err, "returned error \"%v\" does not match expected \"%s\"", err, test.err)
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	ni            *oc.NetworkInstance
}

func (f *FakeFeature) AugmentNetworkInstance(ni *oc.NetworkInstance) error {
	f.ni = ni
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
	tests := []struct {
		desc string
		err  error
	}{{
		desc: "error not expected",
	}, {
		desc: "error expected",
		err:  errors.New("some error"),
	}}

	for _, test := range tests {
		ni := New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		ff := &FakeFeature{Err: test.err}
		gotErr := ni.WithFeature(ff)
		assert.True(t, ff.augmentCalled, "AugmentNetworkInstance was not called")
		assert.Equal(t, ff.ni, ni.oc, "NI ptr is not equal")
		assert.ErrorIs(t, gotErr, test.err, "Error strings are not equal")
	}
}
