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

// Package bgp implements the Config Library for BGP base feature profile.
package bgp

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Name of the BGP protocol.
const Name = "bgp"

// BGP struct stores the OC attributes for BGP base feature profile.
type BGP struct {
	oc fpoc.NetworkInstance_Protocol
}

// New returns a new BGP object.
func New() *BGP {
	return &BGP{
		oc: fpoc.NetworkInstance_Protocol{
			Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			Name:       ygot.String(Name),
		},
	}
}

// WithAS sets the AS value for BGP global.
func (b *BGP) WithAS(as uint32) *BGP {
	b.oc.GetOrCreateBgp().GetOrCreateGlobal().As = ygot.Uint32(as)
	return b
}

// WithAFISAFI sets the AFI-SAFI type for BGP global.
func (b *BGP) WithAFISAFI(name fpoc.E_BgpTypes_AFI_SAFI_TYPE) *BGP {
	b.oc.GetOrCreateBgp().GetOrCreateGlobal().GetOrCreateAfiSafi(name).Enabled = ygot.Bool(true)
	return b
}

// WithRouterID sets the router-id value for BGP global.
func (b *BGP) WithRouterID(rID string) *BGP {
	b.oc.GetOrCreateBgp().GetOrCreateGlobal().RouterId = ygot.String(rID)
	return b
}

// AugmentNetworkInstance implements networkinstance.Feature interface.
// Augments the provided NI with BGP OC.
// Use ni.WithFeature(b) instead of calling this method directly.
func (b *BGP) AugmentNetworkInstance(ni *fpoc.NetworkInstance) error {
	if err := b.oc.Validate(); err != nil {
		return err
	}
	p := ni.GetProtocol(b.oc.GetIdentifier(), Name)
	if p == nil {
		return ni.AppendProtocol(&b.oc)
	}
	return ygot.MergeStructInto(p, &b.oc)
}

// GlobalFeature provides interface to augment BGP with additional features.
type GlobalFeature interface {
	// AugmentBGP augments BGP with additional features.
	AugmentBGP(oc *fpoc.NetworkInstance_Protocol_Bgp) error
}

// WithFeature augments BGP with provided feature.
func (b *BGP) WithFeature(f GlobalFeature) error {
	return f.AugmentBGP(b.oc.GetBgp())
}
