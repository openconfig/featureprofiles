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

package device

import (
	"errors"
	"testing"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/stretchr/testify/assert"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	d := New()
	assert.NotNil(t, d, "New returned nil")
	assert.NotNil(t, d.oc, "Device OC is nil")
}

// TestDeepCopy tests the DeepCopy method.
func TestDeepCopy(t *testing.T) {
	d := New()
	dc, err := d.DeepCopy()
	assert.NoError(t, err, "DeepCopy returned error %v", err)
	assert.NotNil(t, dc, "DeepCopy returned nil")
	// ygot library implements a thorough test for DeepCopy
	// and hence we don't need to repeat that again.
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	d             *oc.Device
}

func (f *FakeFeature) AugmentDevice(d *oc.Device) error {
	f.d = d
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
		d := New()
		ff := &FakeFeature{Err: test.err}
		gotErr := d.WithFeature(ff)
		assert.True(t, ff.augmentCalled, "AugmentDevice was not called")
		assert.Equal(t, ff.d, d.oc, "Device ptr is not equal")
		assert.ErrorIs(t, gotErr, test.err, "Error strings are not equal")
	}
}
