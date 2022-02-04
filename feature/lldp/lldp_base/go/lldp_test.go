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

package lldp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	want := &LLDP{
		oc: &oc.Lldp{
			Enabled: ygot.Bool(true),
		},
	}
	got := New()
	assert.NotNil(t, got, "New returned nil")
	if diff := cmp.Diff(want.oc, got.oc, protocmp.Transform()); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithInterface tests the WithInterface function.
func TestWithInterface(t *testing.T) {
	tests := []struct {
		desc  string
		intfs []string
	}{{
		desc:  "one interface",
		intfs: []string{"Ethernet1.1"},
	}, {
		desc:  "multiple interfaces",
		intfs: []string{"Ethernet1.1", "Ethernet1.2"},
	}}
	for _, test := range tests {
		l := &LLDP{
			oc: &oc.Lldp{
				Enabled: ygot.Bool(true),
			},
		}
		dcopy, err := ygot.DeepCopy(l.oc)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		want := dcopy.(*oc.Lldp)
		for _, iname := range test.intfs {
			want.GetOrCreateInterface(iname).Enabled = ygot.Bool(true)
			got := l.WithInterface(iname)
			assert.NotNil(t, got, "New returned nil")
		}

		if diff := cmp.Diff(want, l.oc, protocmp.Transform()); diff != "" {
			t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
		}
	}
}

// TestAugmentDevice tests the NI augment to device OC.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc    string
		device  *oc.Device
		wantErr bool
	}{{
		desc:   "empty device",
		device: &oc.Device{},
	}, {
		desc: "device contains lldp",
		device: func() *oc.Device {
			d := &oc.Device{}
			d.GetOrCreateLldp().Enabled = ygot.Bool(true)
			return d
		}(),
		wantErr: true,
	}}

	for _, test := range tests {
		l := &LLDP{
			oc: &oc.Lldp{
				Enabled: ygot.Bool(true),
			},
		}
		dcopy, err := ygot.DeepCopy(test.device)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		wantDevice := dcopy.(*oc.Device)

		err = l.AugmentDevice(test.device)
		if test.wantErr {
			assert.Error(t, err, "error expected")
		} else {
			assert.NoError(t, err, "error not expected")
			wantDevice.Lldp = l.oc
			if diff := cmp.Diff(wantDevice, test.device, protocmp.Transform()); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		}
	}
}
