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

// Package staticroute implements the Config Library for StaticRoute
// feature.
package staticroute

import (
	"strconv"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Name of the protocol
const Name = "static"

// Static struct stores the OC attributes for the  base feature profile.
type Static struct {
	oc fpoc.NetworkInstance_Protocol
}

// New returns a new Static object.
func New() *Static {
	return &Static{
		oc: fpoc.NetworkInstance_Protocol{
			Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       ygot.String(Name),
		},
	}
}

// WithRoute sets the prefix with next-hops for static route.
func (sr *Static) WithRoute(prefix string, nextHops []string) *Static {
	static := sr.oc.GetOrCreateStatic(prefix)
	for i, nh := range nextHops {
		str := strconv.Itoa(i + 1)
		n := static.GetOrCreateNextHop(str)
		n.NextHop = fpoc.UnionString(nh)
	}
	return sr
}

// AugmentNetworkInstance implements networkinstance.Feature interface.
// Augments the provided NI with Static OC.
func (sr *Static) AugmentNetworkInstance(ni *fpoc.NetworkInstance) error {
	if err := sr.oc.Validate(); err != nil {
		return err
	}
	p := ni.GetProtocol(sr.oc.GetIdentifier(), Name)
	if p == nil {
		return ni.AppendProtocol(&sr.oc)
	}
	return ygot.MergeStructInto(p, &sr.oc)
}
