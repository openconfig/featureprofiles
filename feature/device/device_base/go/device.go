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
	"errors"
	"fmt"

	"github.com/openconfig/featureprofiles/yang/oc"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
)

// Device struct to store OC attributes.
type Device struct {
	oc *oc.Device
}

// New returns a new Device objct.
func New() *Device {
	return &Device{oc: &oc.Device{}}
}

// DeepCopy returns a deep copy of Device OC struct.
func (d *Device) DeepCopy() (*oc.Device, error) {
	dcopy, err := ygot.DeepCopy(d.oc)
	if err != nil {
		return nil, err
	}
	return dcopy.(*oc.Device), nil
}

// Merge merges the provided Device into this object.
func (d *Device) Merge(src *Device) error {
	return ygot.MergeStructInto(d.oc, src.oc)
}

// EmitJSON returns the config in RFC7951 JSON format.
func (d *Device) EmitJSON() (string, error) {
	b, err := ygot.EmitJSON(d.oc, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		Indent: "  ",
		ValidationOpts: []ygot.ValidationOption{
			&ytypes.LeafrefOptions{
				IgnoreMissingData: true,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("ygot.EmitJSON => error: %v", err)
	}
	return string(b), nil
}

// FullReplace returns gNMI SetRequest with full config replace at root node.
func (d *Device) FullReplace() (*gnmipb.SetRequest, error) {
	// gNMI Root path "/"
	rootPath := gnmipb.Path{
		Origin: "openconfig",
		Elem:   []*gnmipb.PathElem{},
	}

	val, err := ygot.EncodeTypedValue(d.oc, gnmipb.Encoding_JSON_IETF)
	if err != nil {
		return nil, err
	}
	r := &gnmipb.SetRequest{
		Replace: []*gnmipb.Update{
			&gnmipb.Update{
				Path: &rootPath,
				Val:  val,
			},
		},
	}
	return r, nil
}

// DeviceFeature is a feature on the device.
type DeviceFeature interface {
	// AugmentDevice augments the device OC with this feature.
	AugmentDevice(d *oc.Device) error
}

// WithFeature augments the device with the provided feature.
func (d *Device) WithFeature(f DeviceFeature) error {
	if f == nil {
		return errors.New("feature is nil")
	}
	return f.AugmentDevice(d.oc)
}
