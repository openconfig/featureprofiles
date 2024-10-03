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
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// NewSFlowGlobalCfg takes optional input of sflow global and sfcollector and returns OC
// configuration including any deviations for the device.
// If sfglobal is nil, default values are provided.
// The SFlow configuration is returned to give the caller an option to override default values.
func NewSFlowGlobalCfg(t *testing.T, batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow, d *ondatra.DUTDevice, ni, intfName string) *oc.Sampling_Sflow {
	c := new(oc.Sampling_Sflow)

	if newcfg == nil {
		c.Enabled = ygot.Bool(true)
		c.SampleSize = ygot.Uint16(256)
		c.IngressSamplingRate = ygot.Uint32(1000000)
		// c.EgressSamplingRate = ygot.Uint32(1000000),  TODO: verify if EgressSamplingRate is a required DUT feature
		c.Dscp = ygot.Uint8(8)
		c.GetOrCreateInterface(d.Port(t, "port1").Name()).Enabled = ygot.Bool(true)
		c.GetOrCreateInterface(d.Port(t, "port2").Name()).Enabled = ygot.Bool(true)
		coll := NewSFlowCollector(t, batch, nil, d, ni, intfName)
		for _, col := range coll {
			c.AppendCollector(col)
		}
	} else {
		*c = *newcfg
	}

	gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Config(), c)

	return c
}

// NewSFlowCollector creates a collector to be appended to SFlowConfig.
// If sfc is nil, default values are provided.
func NewSFlowCollector(t *testing.T, batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow_Collector, d *ondatra.DUTDevice, ni, intfName string) []*oc.Sampling_Sflow_Collector {
	coll := []*oc.Sampling_Sflow_Collector{}

	if newcfg == nil {
		intf := gnmi.Get[*oc.Interface](t, d, gnmi.OC().Interface(intfName).State())

		cV4 := new(oc.Sampling_Sflow_Collector)
		cV4.SetAddress("192.0.2.129")
		cV4.SetPort(6343)

		if deviations.SflowSourceAddressUpdateUnsupported(d) {
			sFlowSourceAddressCli := ""
			switch d.Vendor() {
			case ondatra.ARISTA:
				sFlowSourceAddressCli = fmt.Sprintf("sflow vrf %s source-interface %s", ni, intf.GetName())
			}
			if sFlowSourceAddressCli != "" {
				helpers.GnmiCLIConfig(t, d, sFlowSourceAddressCli)
			}
		} else {
			addresses := []string{}
			for _, addr := range intf.GetSubinterface(0).GetIpv4().Address {
				addresses = append(addresses, addr.GetIp())
			}
			cV4.SetSourceAddress(addresses[0])
		}
		cV4.SetNetworkInstance(ni)
		coll = append(coll, cV4)

		cV6 := new(oc.Sampling_Sflow_Collector)
		cV6.SetAddress("2001:db8::129")
		cV6.SetPort(6343)
		if deviations.SflowSourceAddressUpdateUnsupported(d) {
			sFlowSourceAddressCli := ""
			switch d.Vendor() {
			case ondatra.ARISTA:
				sFlowSourceAddressCli = fmt.Sprintf("sflow vrf %s source-interface %s", ni, intf.GetName())
			}
			if sFlowSourceAddressCli != "" {
				helpers.GnmiCLIConfig(t, d, sFlowSourceAddressCli)
			}
		} else {
			addresses := []string{}
			for _, addr := range intf.GetSubinterface(0).GetIpv6().Address {
				addresses = append(addresses, addr.GetIp())
			}
			cV6.SetSourceAddress(addresses[0])
		}
		cV6.SetNetworkInstance(ni)
		coll = append(coll, cV6)
	} else {
		coll = append(coll, newcfg)
	}

	for _, c := range coll {
		gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Collector(c.GetAddress(), c.GetPort()).Config(), c)
	}

	return coll
}
