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

package isis

import (
	"time"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// InterfaceLevel struct to hold ISIS interface level OC attributes.
type InterfaceLevel struct {
	oc fpoc.NetworkInstance_Protocol_Isis_Interface_Level
}

// NewInterfaceLevel returns a new InterfaceLevel object.
func NewInterfaceLevel(levelNum int) *InterfaceLevel {
	return &InterfaceLevel{
		oc: fpoc.NetworkInstance_Protocol_Isis_Interface_Level{
			Enabled:     ygot.Bool(true),
			LevelNumber: ygot.Uint8(uint8(levelNum)),
		},
	}
}

// WithHelloInterval sets the hello-interval on the interface ISIS level.
func (il *InterfaceLevel) WithHelloInterval(interval time.Duration) *InterfaceLevel {
	toc := il.oc.GetOrCreateTimers()
	toc.HelloInterval = ygot.Uint32(uint32(interval.Seconds()))
	return il
}

// WithHelloMultiplier sets the hello-interval on the interface ISIS level.
func (il *InterfaceLevel) WithHelloMultiplier(m int) *InterfaceLevel {
	toc := il.oc.GetOrCreateTimers()
	toc.HelloMultiplier = ygot.Uint8(uint8(m))
	return il
}

// WithAFISAFIMetric sets the AFI-SAFI metric for interface ISIS level.
func (il *InterfaceLevel) WithAFISAFIMetric(afi fpoc.E_IsisTypes_AFI_TYPE, safi fpoc.E_IsisTypes_SAFI_TYPE, metric int) *InterfaceLevel {
	aoc := il.oc.GetOrCreateAf(afi, safi)
	aoc.Enabled = ygot.Bool(true)
	aoc.Metric = ygot.Uint32(uint32(metric))
	return il
}

// AugmentInterface implements the isis.InterfaceFeature interface.
// This method augments the ISIS interface OC with level configuration.
// Use isis.WithFeature(il) instead of calling this method directly.
func (il *InterfaceLevel) AugmentInterface(isisIntf *fpoc.NetworkInstance_Protocol_Isis_Interface) error {
	if err := il.oc.Validate(); err != nil {
		return err
	}
	ioc := isisIntf.GetLevel(il.oc.GetLevelNumber())
	if ioc == nil {
		return isisIntf.AppendLevel(&il.oc)
	}
	return ygot.MergeStructInto(ioc, &il.oc)
}
