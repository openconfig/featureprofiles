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

// Package ntp implements the Config Library for System NTP feature profile.
package ntp

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// NTP struct stores the OC attributes for System NTP feature profile.
type NTP struct {
	oc fpoc.System_Ntp
}

// New returns a new NTP object.
func New() *NTP {
	return &NTP{
		oc: fpoc.System_Ntp{
			Enabled: ygot.Bool(true),
		},
	}
}

// AddServer adds a new server with address and port.
func (n *NTP) AddServer(address string, port int) *NTP {
	n.oc.GetOrCreateServer(address).Port = ygot.Uint16(uint16(port))
	return n
}

// AugmentSystem implements the system.SystemFeature interface.
// This method augments the System with NTP feature.
// Use s.WithFeature(n) instead of calling this method directly.
func (n *NTP) AugmentSystem(s *fpoc.System) error {
	if err := n.oc.Validate(); err != nil {
		return err
	}
	if s.Ntp == nil {
		s.Ntp = &n.oc
		return nil
	}
	return ygot.MergeStructInto(s.Ntp, &n.oc)
}
