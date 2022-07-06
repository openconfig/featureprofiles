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

// Package isis implements the Config Library for ISIS base feature profile.
package isis

import (
	"time"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Name of the ISIS protocol.
const Name = "isis"

// ISIS struct stores the OC attributes for ISIS base feature profile.
type ISIS struct {
	oc fpoc.NetworkInstance_Protocol
}

// New returns a new ISIS object.
func New() *ISIS {
	return &ISIS{
		oc: fpoc.NetworkInstance_Protocol{
			Identifier: fpoc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
			Name:       ygot.String(Name),
		},
	}
}

// WithNet sets the Net value for ISIS global.
func (i *ISIS) WithNet(net string) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().Net = []string{net}
	return i
}

// WithAFISAFI sets the AFI-SAFI type for BGP global.
func (i *ISIS) WithAFISAFI(afi fpoc.E_IsisTypes_AFI_TYPE, safi fpoc.E_IsisTypes_SAFI_TYPE) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateAf(afi, safi).Enabled = ygot.Bool(true)
	return i
}

// WithLevelCapability sets the level-capability for ISIS global.
func (i *ISIS) WithLevelCapability(level fpoc.E_IsisTypes_LevelType) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().LevelCapability = level
	return i
}

// WithLSPMTUSize sets the LSP MTU size for ISIS global.
func (i *ISIS) WithLSPMTUSize(size int) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateTransport().LspMtuSize = ygot.Uint16(uint16(size))
	return i
}

// WithLSPLifetimeInterval sets the lsp-lifetime-interval for ISIS global.
func (i *ISIS) WithLSPLifetimeInterval(interval time.Duration) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateTimers().LspLifetimeInterval = ygot.Uint16(uint16(interval.Seconds()))
	return i
}

// WithLSPRefreshInterval sets the lsp-refresh-interval for ISIS global.
func (i *ISIS) WithLSPRefreshInterval(interval time.Duration) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateTimers().LspRefreshInterval = ygot.Uint16(uint16(interval.Seconds()))
	return i
}

// WithSPFFirstInterval sets the spf-first-interval for ISIS global.
func (i *ISIS) WithSPFFirstInterval(interval time.Duration) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateTimers().GetOrCreateSpf().SpfFirstInterval = ygot.Uint64(uint64(interval.Milliseconds()))
	return i
}

// WithSPFHoldInterval sets the spf-hold-interval for ISIS global.
func (i *ISIS) WithSPFHoldInterval(interval time.Duration) *ISIS {
	i.oc.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateTimers().GetOrCreateSpf().SpfHoldInterval = ygot.Uint64(uint64(interval.Milliseconds()))
	return i
}

// AugmentNetworkInstance implements networkinstance.Feature interface.
// Augments the provided NI with ISIS OC.
// Use ni.WithFeature(i) instead of calling this method directly.
func (i *ISIS) AugmentNetworkInstance(ni *fpoc.NetworkInstance) error {
	if err := i.oc.Validate(); err != nil {
		return err
	}
	p := ni.GetProtocol(i.oc.GetIdentifier(), Name)
	if p == nil {
		return ni.AppendProtocol(&i.oc)
	}
	return ygot.MergeStructInto(p, &i.oc)
}

// GlobalFeature provides interface to augment ISIS with additional features.
type GlobalFeature interface {
	// AugmentISIS augments ISIS with additional features.
	AugmentISIS(oc *fpoc.NetworkInstance_Protocol_Isis) error
}

// WithFeature augments ISIS with provided feature.
func (i *ISIS) WithFeature(f GlobalFeature) error {
	return f.AugmentISIS(i.oc.GetIsis())
}
