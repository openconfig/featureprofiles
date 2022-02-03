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

// Package lldp implements the Config Library for LLDP feature profile.
package lldp

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// To enable LLDP on a device:
//
// device.New().WithFeature(lldp.New())

// LLDP struct to store OC attributes.
type LLDP struct {
	oc *oc.Lldp
}

// New returns a new LLDP object with the feature enabled.
func New() *LLDP {
	oc := &oc.Lldp{
		Enabled: ygot.Bool(true),
	}
	return &LLDP{oc: oc}
}

// WithInterface enables LLDP on the specified interface.
func (l *LLDP) WithInterface(name string) *LLDP {
	if l == nil {
		return nil
	}
	l.oc.GetOrCreateInterface(name).Enabled = ygot.Bool(true)
	return l
}

// AugmentDevice augments the device OC with LLDP feature.
// Use d.WithFeature(l) instead of calling this method directly.
func (l *LLDP) AugmentDevice(d *oc.Device) error {
	if l == nil || d == nil {
		return errors.New("some args are nil")
	}
	if d.Lldp != nil {
		return errors.New("lldp OC is not nil")
	}
	d.Lldp = l.oc
	return nil
}
