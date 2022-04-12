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
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// LLDP struct to store OC attributes.
type LLDP struct {
	oc fpoc.Lldp
}

// New returns a new LLDP object with the feature enabled.
func New() *LLDP {
	return &LLDP{
		oc: fpoc.Lldp{
			Enabled: ygot.Bool(true),
		},
	}
}

// EnableInterface enables LLDP on the specified interface.
func (l *LLDP) EnableInterface(name string) *LLDP {
	l.oc.GetOrCreateInterface(name).Enabled = ygot.Bool(true)
	return l
}

// AugmentDevice implements the device.Feature interface.
// This method augments the device OC with LLDP feature.
// Use d.WithFeature(l) instead of calling this method directly.
func (l *LLDP) AugmentDevice(d *fpoc.Device) error {
	if err := l.oc.Validate(); err != nil {
		return err
	}
	if d.Lldp == nil {
		d.Lldp = &l.oc
		return nil
	}
	return ygot.MergeStructInto(d.Lldp, &l.oc)
}
