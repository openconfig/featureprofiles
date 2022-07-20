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

package sflow

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Interface struct to hold Sflow interface OC attributes.
type Interface struct {
	oc fpoc.Sampling_Sflow_Interface
}

// NewInterface returns a new Interface object.
func NewInterface(name string) *Interface {
	return &Interface{
		oc: fpoc.Sampling_Sflow_Interface{
			Name:    ygot.String(name),
			Enabled: ygot.Bool(true),
		},
	}
}

// AugmentSflow implements the sflow.Feature interface.
// This method augments the Sflow OC with interface configuration.
// Use sflow.WithFeature(i) instead of calling this method directly.
func (i *Interface) AugmentSflow(sflow *fpoc.Sampling_Sflow) error {
	if err := i.oc.Validate(); err != nil {
		return err
	}
	ioc := sflow.GetInterface(i.oc.GetName())
	if ioc == nil {
		return sflow.AppendInterface(&i.oc)
	}
	return ygot.MergeStructInto(ioc, &i.oc)
}
