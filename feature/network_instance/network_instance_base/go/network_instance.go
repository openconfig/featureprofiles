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
)

// To enable default VRF on a device:
//
// device.New()
//     .WithFeature(networkinstance.New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE))
//

// NetworkInstance struct stores the OC attributes.
type NetworkInstance struct {
	oc *oc.NetworkInstance
}

// New returns the new NetworkInstance object with specified name and type.
func New(name string, niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE) *NetworkInstance {
	oc := &oc.NetworkInstance{
		Name:    ygot.String(name),
		Type:    niType,
		Enabled: ygot.Bool(true),
	}
	return &NetworkInstance{oc: oc}
}

// validate method performs some sanity checks.
func (ni *NetworkInstance) validate() error {
	if ni.oc.GetName() == "" {
		return errors.New("NetworkInstance name is empty")
	}
	if ni.oc.GetType() == oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_UNSET {
		return errors.New("NetworkInstance type is unset")
	}
	return nil
}

// AugmentDevice method augments the provided device OC with NetworkInstance
// feature.
// Use d.WithFeature(ni) instead of calling this method directly.
func (ni *NetworkInstance) AugmentDevice(d *oc.Device) error {
	if ni == nil || d == nil {
		return errors.New("either ni or device is nil")
	}
	if err := ni.validate(); err != nil {
		return err
	}
	return d.AppendNetworkInstance(ni.oc)
}

// NetworkInstanceFeature provides interface to augment additional features to NI.
type NetworkInstanceFeature interface {
	// AugmentNetworkInstancd augments NI with provided feature.
	AugmentNetworkInstance(ni *oc.NetworkInstance) error
}

// WithFeature augments the provided feature to network-instance.
func (ni *NetworkInstance) WithFeature(f NetworkInstanceFeature) error {
	if ni == nil || f == nil {
		return errors.New("some args are nil")
	}
	return f.AugmentNetworkInstance(ni.oc)
}
