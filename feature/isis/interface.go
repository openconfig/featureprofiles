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

// Interface struct to hold ISIS interface OC attributes.
type Interface struct {
	oc fpoc.NetworkInstance_Protocol_Isis_Interface
}

// NewInterface returns a new Interface object.
func NewInterface(interfaceID string) *Interface {
	return &Interface{
		oc: fpoc.NetworkInstance_Protocol_Isis_Interface{
			Enabled:     ygot.Bool(true),
			InterfaceId: ygot.String(interfaceID),
		},
	}
}

// WithCircuitType sets the circuit-type on the interface ISIS.
func (i *Interface) WithCircuitType(ct fpoc.E_IsisTypes_CircuitType) *Interface {
	i.oc.CircuitType = ct
	return i
}

// WithCSNPInterval sets the csnp-interval on the interface ISIS.
func (i *Interface) WithCSNPInterval(interval time.Duration) *Interface {
	toc := i.oc.GetOrCreateTimers()
	toc.CsnpInterval = ygot.Uint16(uint16(interval.Seconds()))
	return i
}

// WithLSPPacingInterval sets the lsp-pacing-interval on the interface ISIS.
func (i *Interface) WithLSPPacingInterval(interval time.Duration) *Interface {
	toc := i.oc.GetOrCreateTimers()
	toc.LspPacingInterval = ygot.Uint64(uint64(interval.Milliseconds()))
	return i
}

// WithAFISAFI sets the AFI-SAFI for interface ISIS.
func (i *Interface) WithAFISAFI(afi fpoc.E_IsisTypes_AFI_TYPE, safi fpoc.E_IsisTypes_SAFI_TYPE) *Interface {
	i.oc.GetOrCreateAf(afi, safi).Enabled = ygot.Bool(true)
	return i
}

// AugmentGlobal implements the isis.GlobalFeature interface.
// This method augments the ISIS OC with level configuration.
// Use isis.WithFeature(l) instead of calling this method directly.
func (i *Interface) AugmentGlobal(isis *fpoc.NetworkInstance_Protocol_Isis) error {
	if err := i.oc.Validate(); err != nil {
		return err
	}
	ioc := isis.GetInterface(i.oc.GetInterfaceId())
	if ioc == nil {
		return isis.AppendInterface(&i.oc)
	}
	return ygot.MergeStructInto(ioc, &i.oc)
}

// InterfaceFeature provides interface to augment ISIS interface with
// additional features.
type InterfaceFeature interface {
	// AugmentInterface augments ISIS interface with additional features.
	AugmentInterface(oc *fpoc.NetworkInstance_Protocol_Isis_Interface) error
}

// WithFeature augments ISIS interface with provided feature.
func (i *Interface) WithFeature(f InterfaceFeature) error {
	return f.AugmentInterface(&i.oc)
}
