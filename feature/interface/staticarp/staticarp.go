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

// Package staticarp implements the Config Library for staticarp feature profile.
package staticarp

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// StaticARP struct to store OC attributes.
type StaticARP struct {
	v4oc fpoc.Interface_Subinterface_Ipv4
	v6oc fpoc.Interface_Subinterface_Ipv6
}

// New returns a new StaticARP object with the feature enabled.
func New() *StaticARP {
	return &StaticARP{}
}

// AddIPv4Address sets ipv4 address on the specified interface.
func (s *StaticARP) AddIPv4Address(addr string, prefixlen int) *StaticARP {
	s.v4oc.GetOrCreateAddress(addr).PrefixLength = ygot.Uint8(uint8(prefixlen))
	return s
}

// AddIPv6Address sets ipv6 address on the specified interface.
func (s *StaticARP) AddIPv6Address(addr string, prefixlen int) *StaticARP {
	s.v6oc.GetOrCreateAddress(addr).PrefixLength = ygot.Uint8(uint8(prefixlen))
	return s
}

// AddIPv4Neighbor add ipv4 neighbor on the specified interface.
func (s *StaticARP) AddIPv4Neighbor(ip string, mac string) *StaticARP {
	s.v4oc.GetOrCreateNeighbor(ip).LinkLayerAddress = ygot.String(mac)
	return s
}

// AddIPv6Neighbor add ipv6 neighbor on the specified interface.
func (s *StaticARP) AddIPv6Neighbor(ip string, mac string) *StaticARP {
	s.v6oc.GetOrCreateNeighbor(ip).LinkLayerAddress = ygot.String(mac)
	return s
}

// AugmentSubInterface implements the device.Feature interface.
// This method augments the device OC with Interface feature.
// Use d.WithFeature(i) instead of calling this method directly.
func (s *StaticARP) AugmentSubInterface(subintf *fpoc.Interface_Subinterface) error {
	// Augment sub-interface with v4 staticarp.
	if err := s.v4oc.Validate(); err != nil {
		return err
	}
	v4oc := subintf.GetIpv4()
	if v4oc == nil {
		subintf.Ipv4 = &s.v4oc
	} else {
		if err := ygot.MergeStructInto(v4oc, &s.v4oc); err != nil {
			return err
		}
	}

	// Augment sub-interface with v6 staticarp.
	if err := s.v6oc.Validate(); err != nil {
		return err
	}
	v6oc := subintf.GetIpv6()
	if v6oc == nil {
		subintf.Ipv6 = &s.v6oc
	} else {
		if err := ygot.MergeStructInto(v6oc, &s.v6oc); err != nil {
			return err
		}
	}
	return nil
}
