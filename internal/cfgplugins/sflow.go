// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// NewSFlowGlobalCfg takes optional input of sflow global and sfcollector and returns OC
// configuration including any deviations for the device.
// If sfglobal is nil, default values are provided.
// The SFlow configuration is returned to give the caller an option to override default values.
func NewSFlowGlobalCfg(batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow, d *ondatra.DUTDevice) *oc.Sampling_Sflow {
	c := new(oc.Sampling_Sflow)

	if newcfg == nil {
		c.Enabled = ygot.Bool(true)
		c.SampleSize = ygot.Uint16(256)
		c.IngressSamplingRate = ygot.Uint32(1000000)
		// c.EgressSamplingRate = ygot.Uint32(1000000),  TODO: verify if EgressSamplingRate is a required DUT feature
		c.Dscp = ygot.Uint8(8)
		coll := new(oc.Sampling_Sflow_Collector)
		coll.SetAddress("192.0.2.129")
		coll.SetPort(6343)
		coll.SetSourceAddress("192.0.2.5")
		coll.SetNetworkInstance(deviations.DefaultNetworkInstance(d))
		c.AppendCollector(coll)
	} else {
		*c = *newcfg
	}

	gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Config(), c)

	return c
}

// NewSFlowCollector creates a collector to be appended to SFlowConfig.
// If sfc is nil, default values are provided.
func NewSFlowCollector(batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow_Collector, d *ondatra.DUTDevice) *oc.Sampling_Sflow_Collector {
	c := new(oc.Sampling_Sflow_Collector)

	if newcfg == nil {
		c.SetAddress("192.0.2.129")
		c.SetPort(6343)
		c.SetSourceAddress("192.0.2.5")
	} else {
		*c = *newcfg
	}

	c.SetNetworkInstance(normalizeNIName("DEFAULT", d))
	gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Collector(c.GetAddress(), c.GetPort()).Config(), c)

	return c
}
