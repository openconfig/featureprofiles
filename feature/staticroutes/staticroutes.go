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

// Package networkinstance implements the Config Library for NetworkInstance
// feature.
package networkinstance

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
	"strconv"
)

// NetworkInstance struct stores the OC attributes.
type NetworkInstance struct {
	oc oc.NetworkInstance
}

// validate method performs some sanity checks.
func (ni *NetworkInstance) validate() error {
	p := ni.oc.GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static")
		for _, v := p.Static {
			for _, nh := v.NextHop {
				if nh == "" {
					return errors.New("NetworkInstance nexthop is unset")
				}
			}
		}
	}
	return ni.oc.Validate()
}

// WithStaticRoute sets the prefix value for static route.
func (ni *NetworkInstance) WithStaticRoute(prefix string, nextHops []string) *NetworkInstance {
	static := ni.oc.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static").GetOrCreateStatic(prefix)
	static.Prefix = ygot.String(prefix)
	for i, nh := range nextHops {
		str := strconv.Itoa(i + 1)
		n := static.GetOrCreateNextHop(str)
		n.NextHop = oc.UnionString(nh)
	}
	return ni
}

// AugmentDevice implements the device.Feature interface.
// This method augments the provided device OC with NetworkInstance feature.
// Use d.WithFeature(ni) instead of calling this method directly.
func (ni *NetworkInstance) AugmentDevice(d *oc.Device) error {
	if err := ni.validate(); err != nil {
		return err
	}
	deviceNI := d.GetNetworkInstance(ni.oc.GetName())
	if deviceNI == nil {
		return d.AppendNetworkInstance(&ni.oc)
	}
	return ygot.MergeStructInto(deviceNI, &ni.oc)
}
