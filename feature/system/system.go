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

// Package system implements the Config Library for System base feature profile.
package system

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// System struct stores the OC attributes for System base feature profile.
type System struct {
	oc fpoc.System
}

// New returns a new System object.
func New() *System {
	return &System{}
}

// WithHostname sets the hostname value.
func (s *System) WithHostname(name string) *System {
	s.oc.Hostname = ygot.String(name)
	return s
}

// WithDomainName sets the domain-name value.
func (s *System) WithDomainName(name string) *System {
	s.oc.DomainName = ygot.String(name)
	return s
}

// WithLoginBanner sets the login-banner value.
func (s *System) WithLoginBanner(banner string) *System {
	s.oc.LoginBanner = ygot.String(banner)
	return s
}

// WithMOTDBanner sets the motd-banner value.
func (s *System) WithMOTDBanner(banner string) *System {
	s.oc.MotdBanner = ygot.String(banner)
	return s
}

// WithTimezoneName sets the timezone-name value.
func (s *System) WithTimezoneName(name string) *System {
	s.oc.GetOrCreateClock().TimezoneName = ygot.String(name)
	return s
}

// AddUserWithSSHKey adds a user with SSH key.
func (s *System) AddUserWithSSHKey(username, sshkey string) *System {
	s.oc.GetOrCreateAaa().GetOrCreateAuthentication().GetOrCreateUser(username).SshKey = ygot.String(sshkey)
	return s
}

// AugmentDevice implements the device.Feature interface.
// This method augments the provided device OC with System feature.
// Use d.WithFeature(s) instead of calling this method directly.
func (s *System) AugmentDevice(d *fpoc.Device) error {
	deviceSystem := d.GetSystem()
	if deviceSystem == nil {
		d.System = &s.oc
		return nil
	}
	return ygot.MergeStructInto(deviceSystem, &s.oc)
}

// Feature provides interface to augment System with additional features.
type Feature interface {
	// AugmentSystem augments System with additional features.
	AugmentSystem(oc *fpoc.System) error
}

// WithFeature augments System with provided feature.
func (s *System) WithFeature(f Feature) error {
	return f.AugmentSystem(&s.oc)
}
