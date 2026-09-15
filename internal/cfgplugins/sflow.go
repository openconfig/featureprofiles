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

// SFlowGlobalParams defines parameters for the SFlow global configuration.
type SFlowGlobalParams struct {
	Ni              string
	IntfName        string
	SrcAddrV4       string
	SrcAddrV6       string
	IP              string
	MinSamplingRate uint32
	Egress          bool
}

// NewSFlowGlobalCfg takes optional input of sflow global and sfcollector and returns OC
// configuration including any deviations for the device.
// If sfglobal is nil, default values are provided.
// The SFlow configuration is returned to give the caller an option to override default values.
func NewSFlowGlobalCfg(t *testing.T, batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow, d *ondatra.DUTDevice, p *SFlowGlobalParams) *oc.Sampling_Sflow {
	c := new(oc.Sampling_Sflow)
	if newcfg == nil {
		c.Enabled = ygot.Bool(true)
		c.SampleSize = ygot.Uint16(256)
		c.Dscp = ygot.Uint8(8)
		if p.Egress {
			port2Name := d.Port(t, "port2").Name()
			intfCfg := c.GetOrCreateInterface(port2Name)
			intfCfg.Enabled = ygot.Bool(true)
			if deviations.SflowEgressSamplingRateUnsupported(d) {
				// Arista EOS does not support /sampling/sflow/interfaces/interface/config/egress-sampling-rate
				// over OpenConfig gNMI (b/562517133). Instead, sampling rate is configured globally via
				// /sampling/sflow/config/ingress-sampling-rate (CLI: sflow sample <rate>) and egress sampling
				// is enabled per-interface via CLI (sflow egress enable).
				c.SetIngressSamplingRate(p.MinSamplingRate)
				switch d.Vendor() {
				case ondatra.ARISTA:
					helpers.GnmiCLIConfig(t, d, fmt.Sprintf("interface %s\nsflow egress enable", port2Name))
				}
			} else {
				intfCfg.SetEgressSamplingRate(p.MinSamplingRate)
			}
		} else {
			// override ingress sampling rate if default value of 1000000 is not supported
			if deviations.SflowIngressMinSamplingRate(d) != 0 {
				switch d.Vendor() {
				case ondatra.CISCO:
					c.SetIngressSamplingRate(deviations.SflowIngressMinSamplingRate(d))
				}
			} else {
				c.SetIngressSamplingRate(p.MinSamplingRate)
			}
			c.GetOrCreateInterface(d.Port(t, "port1").Name()).Enabled = ygot.Bool(true)
			c.GetOrCreateInterface(d.Port(t, "port2").Name()).Enabled = ygot.Bool(true)
		}
		cp := &SFlowCollectorParams{
			Ni:        p.Ni,
			IntfName:  p.IntfName,
			SrcAddrV4: p.SrcAddrV4,
			SrcAddrV6: p.SrcAddrV6,
			IP:        p.IP,
		}
		coll := NewSFlowCollector(t, batch, nil, d, cp)
		for _, col := range coll {
			c.AppendCollector(col)
		}
	} else {
		*c = *newcfg
	}
	gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Config(), c)
	return c
}

// DisableSFlowEgressCfg disables egress sFlow sampling on the specified port.
func DisableSFlowEgressCfg(t *testing.T, d *ondatra.DUTDevice, portName string) {
	t.Helper()
	if deviations.SflowEgressSamplingRateUnsupported(d) {
		switch d.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, d, fmt.Sprintf("interface %s\nno sflow egress enable", portName))
		}
	}
	gnmi.Replace(t, d, gnmi.OC().Sampling().Sflow().Interface(portName).Enabled().Config(), false)
}

// SFlowCollectorParams defines parameters for the SFlow collector configuration.
type SFlowCollectorParams struct {
	Ni        string
	IntfName  string
	SrcAddrV4 string
	SrcAddrV6 string
	IP        string
}

// NewSFlowCollector creates a collector to be appended to SFlowConfig.
// If sfc is nil, default values are provided.
func NewSFlowCollector(t *testing.T, batch *gnmi.SetBatch, newcfg *oc.Sampling_Sflow_Collector, d *ondatra.DUTDevice, p *SFlowCollectorParams) []*oc.Sampling_Sflow_Collector {
	var coll []*oc.Sampling_Sflow_Collector
	if newcfg == nil {
		intf := gnmi.Get[*oc.Interface](t, d, gnmi.OC().Interface(p.IntfName).State())
		var address, srcAddress string
		switch p.IP {
		case "IPv4":
			address = "192.0.2.129"
			srcAddress = p.SrcAddrV4
		case "IPv6":
			address = "2001:db8::129"
			srcAddress = p.SrcAddrV6
		}
		cV := new(oc.Sampling_Sflow_Collector)
		cV.SetAddress(address)
		cV.SetPort(6343)
		if deviations.SflowSourceAddressUpdateUnsupported(d) {
			sFlowSourceAddressCli := ""
			switch d.Vendor() {
			case ondatra.ARISTA:
				sFlowSourceAddressCli = fmt.Sprintf("sflow vrf %s source-interface %s", p.Ni, intf.GetName())
			}
			if sFlowSourceAddressCli != "" {
				helpers.GnmiCLIConfig(t, d, sFlowSourceAddressCli)
			}
		} else {
			cV.SetSourceAddress(srcAddress)
		}
		cV.SetNetworkInstance(p.Ni)
		coll = append(coll, cV)
	} else {
		coll = append(coll, newcfg)
	}
	for _, c := range coll {
		gnmi.BatchReplace(batch, gnmi.OC().Sampling().Sflow().Collector(c.GetAddress(), c.GetPort()).Config(), c)
	}
	return coll
}
