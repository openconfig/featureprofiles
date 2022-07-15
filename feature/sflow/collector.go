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

// Collector struct to hold Sflow collector OC attributes.
type Collector struct {
	oc fpoc.Sampling_Sflow_Collector
}

// NewCollector returns a new Collector object.
func NewCollector(address string, port int) *Collector {
	return &Collector{
		oc: fpoc.Sampling_Sflow_Collector{
			Address: ygot.String(address),
			Port:    ygot.Uint16(uint16(port)),
		},
	}
}

// WithNetworkInstance sets network-instance.
func (c *Collector) WithNetworkInstance(ni string) *Collector {
	c.oc.NetworkInstance = ygot.String(ni)
	return c
}

// AugmentSflow implements the sflow.Feature interface.
// This method augments the Sflow OC with interface configuration.
// Use sflow.WithFeature(i) instead of calling this method directly.
func (c *Collector) AugmentSflow(sflow *fpoc.Sampling_Sflow) error {
	if err := c.oc.Validate(); err != nil {
		return err
	}
	coc := sflow.GetCollector(c.oc.GetAddress(), c.oc.GetPort())
	if coc == nil {
		return sflow.AppendCollector(&c.oc)
	}
	return ygot.MergeStructInto(coc, &c.oc)
}
