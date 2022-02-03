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
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To enable BGP on default NI:
// device.New()
//    .WithFeature(networkinstance.New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
//         .WithFeature(bgp.New()
//              .WithAS(1234)))
//

// BGP struct stores the OC attributes for BGP base feature profile.
type BGP struct {
	oc *oc.NetworkInstance_Protocol
}

// New returns a new BGP object.
func New() *BGP {
	oc := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("bgp"),
	}
	return &BGP{oc: oc}
}

// WithAS sets the AS value for BGP global.
func (b *BGP) WithAS(as uint32) *BGP {
	if b == nil {
		return nil
	}
	b.oc.GetOrCreateBgp().GetOrCreateGlobal().As = ygot.Uint32(as)
	return b
}

// WithRouterID sets the router-id value for BGP global.
func (b *BGP) WithRouterID(rID string) *BGP {
	if b == nil {
		return nil
	}
	b.oc.GetOrCreateBgp().GetOrCreateGlobal().RouterId = ygot.String(rID)
	return b
}

// AugmentNetworkInstance augments the provided NI with BGP.
// Use ni.WithFeature(b) instead of calling this method directly.
func (b *BGP) AugmentNetworkInstance(ni *oc.NetworkInstance) error {
	if b == nil || ni == nil {
		return errors.New("some args are nil")
	}
	return ni.AppendProtocol(b.oc)
}

// BGPFeature provides interface to augment BGP with additional features.
type BGPFeature interface {
	// AugmentBGP augments BGP with additional features.
	AugmentBGP(oc *oc.NetworkInstance_Protocol_Bgp) error
}

// WithFeature augments BGP with provided feature.
func (b *BGP) WithFeature(f BGPFeature) error {
	if b == nil || f == nil {
		return errors.New("some args are nil")
	}
	return f.AugmentBGP(b.oc.GetBgp())
}
