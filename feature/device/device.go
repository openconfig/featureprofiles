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

// Package device implements the Config Library for device feature profile.
package device

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// Device struct to store OC attributes.
type Device struct {
	oc fpoc.Device
}

// New returns a new Device objct.
func New() *Device {
	return &Device{}
}

// DeepCopy returns a deep copy of Device OC struct.
func (d *Device) DeepCopy() (*fpoc.Device, error) {
	dcopy, err := ygot.DeepCopy(&d.oc)
	if err != nil {
		return nil, err
	}
	return dcopy.(*fpoc.Device), nil
}

// Merge merges the provided Device into this object.
func (d *Device) Merge(src *Device) error {
	return ygot.MergeStructInto(&d.oc, &src.oc)
}

// FullReplaceRequest returns gNMI SetRequest with full config replace at root node.
func (d *Device) FullReplaceRequest() (*gnmipb.SetRequest, error) {
	if err := d.oc.Validate(); err != nil {
		return nil, err
	}
	val, err := ygot.EncodeTypedValue(&d.oc, gnmipb.Encoding_JSON_IETF)
	if err != nil {
		return nil, err
	}
	r := &gnmipb.SetRequest{
		Replace: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{
					Origin: "openconfig",
					Elem:   []*gnmipb.PathElem{},
				},
				Val: val,
			},
		},
	}
	return r, nil
}

// Feature is a feature on the device.
type Feature interface {
	// AugmentDevice augments the device OC with this feature.
	AugmentDevice(d *fpoc.Device) error
}

// WithFeature augments the device with the provided feature.
func (d *Device) WithFeature(f Feature) error {
	return f.AugmentDevice(&d.oc)
}
