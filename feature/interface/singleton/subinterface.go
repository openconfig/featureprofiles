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

package intf

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// SubInterface struct to hold sub-interface OC attributes.
type SubInterface struct {
	oc fpoc.Interface_Subinterface
}

// NewSubInterface returns a new SubInterface object.
func NewSubInterface(index int, description string) *SubInterface {
	return &SubInterface{
		oc: fpoc.Interface_Subinterface{
			Index:       ygot.Uint32(uint32(index)),
			Description: ygot.String(description),
			Enabled:     ygot.Bool(true),
		},
	}
}

// WithIPv4Enabled enables IPv4.
func (s *SubInterface) WithIPv4Enabled(en bool) *SubInterface {
	s.oc.GetOrCreateIpv4().Enabled = ygot.Bool(en)
	return s
}

// WithIPv4MTU sets ipv4 mtu.
func (s *SubInterface) WithIPv4MTU(mtu int) *SubInterface {
	s.oc.GetOrCreateIpv4().Mtu = ygot.Uint16(uint16(mtu))
	return s
}

// WithIPv6Enabled enables IPv6.
func (s *SubInterface) WithIPv6Enabled(en bool) *SubInterface {
	s.oc.GetOrCreateIpv6().Enabled = ygot.Bool(en)
	return s
}

// WithIPv6MTU sets ipv4 mtu.
func (s *SubInterface) WithIPv6MTU(mtu int) *SubInterface {
	s.oc.GetOrCreateIpv6().Mtu = ygot.Uint32(uint32(mtu))
	return s
}

// AugmentInterface implements the intf.Feature interface.
// This method augments the Interface OC with subinterface configuration.
// Use intf.WithFeature(s) instead of calling this method directly.
func (s *SubInterface) AugmentInterface(ioc *fpoc.Interface) error {
	if err := s.oc.Validate(); err != nil {
		return err
	}
	soc := ioc.GetSubinterface(s.oc.GetIndex())
	if soc == nil {
		return ioc.AppendSubinterface(&s.oc)
	}
	return ygot.MergeStructInto(soc, &s.oc)
}

// SubInterfaceFeature provides interface to augment SubInterface with additional features.
type SubInterfaceFeature interface {
	// AugmentSubInterface augments SubInterface with additional features.
	AugmentSubInterface(oc *fpoc.Interface_Subinterface) error
}

// WithFeature augments SubInterface with provided feature.
func (s *SubInterface) WithFeature(f SubInterfaceFeature) error {
	return f.AugmentSubInterface(&s.oc)
}
