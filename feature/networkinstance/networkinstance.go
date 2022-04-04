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

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// NetworkInstance struct stores the OC attributes.
type NetworkInstance struct {
	oc fpoc.NetworkInstance
}

// New returns the new NetworkInstance object with specified name and type.
func New(name string, niType fpoc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE) *NetworkInstance {
	return &NetworkInstance{
		oc: fpoc.NetworkInstance{
			Name:    ygot.String(name),
			Type:    niType,
			Enabled: ygot.Bool(true),
		},
	}
}

// validate method performs some sanity checks.
func (ni *NetworkInstance) validate() error {
	if ni.oc.GetName() == "" {
		return errors.New("NetworkInstance name is empty")
	}
	if ni.oc.GetType() == fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_UNSET {
		return errors.New("NetworkInstance type is unset")
	}
	return ni.oc.Validate()
}

// AugmentDevice implements the device.Feature interface.
// This method augments the provided device OC with NetworkInstance feature.
// Use d.WithFeature(ni) instead of calling this method directly.
func (ni *NetworkInstance) AugmentDevice(d *fpoc.Device) error {
	if err := ni.validate(); err != nil {
		return err
	}
	deviceNI := d.GetNetworkInstance(ni.oc.GetName())
	if deviceNI == nil {
		return d.AppendNetworkInstance(&ni.oc)
	}
	return ygot.MergeStructInto(deviceNI, &ni.oc)
}

// Feature provides interface to augment additional features to NI.
type Feature interface {
	// AugmentNetworkInstance augments NI with provided feature.
	AugmentNetworkInstance(ni *fpoc.NetworkInstance) error
}

// WithFeature augments the provided feature to network-instance.
func (ni *NetworkInstance) WithFeature(f Feature) error {
	return f.AugmentNetworkInstance(&ni.oc)
}
