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

// Package intf implements the Config Library for singleton interface feature profile.
package intf

import (
	"time"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Interface struct to store OC attributes.
type Interface struct {
	oc fpoc.Interface
}

// New returns a new Interface object with the feature enabled.
func New(name, description string, t fpoc.E_IETFInterfaces_InterfaceType) *Interface {
	return &Interface{
		oc: fpoc.Interface{
			Name:        ygot.String(name),
			Type:        t,
			Description: ygot.String(description),
			Enabled:     ygot.Bool(true),
		},
	}
}

// WithEnabled sets enabled on the specified interface.
func (i *Interface) WithEnabled(value bool) *Interface {
	i.oc.Enabled = ygot.Bool(value)
	return i
}

// WithForwardingViable sets forwarding-viable on the specified interface.
func (i *Interface) WithForwardingViable(viable bool) *Interface {
	i.oc.ForwardingViable = ygot.Bool(viable)
	return i
}

// WithHoldTimers sets up/down timers on the specified interface.
func (i *Interface) WithHoldTimers(up, down time.Duration) *Interface {
	i.oc.GetOrCreateHoldTime().Up = ygot.Uint32(uint32(up.Milliseconds()))
	i.oc.GetOrCreateHoldTime().Down = ygot.Uint32(uint32(down.Milliseconds()))
	return i
}

// WithMACAddress sets mac-address on the specified interface.
func (i *Interface) WithMACAddress(mac string) *Interface {
	i.oc.GetOrCreateEthernet().MacAddress = ygot.String(mac)
	return i
}

// WithPortSpeed sets port-speed on the specified interface.
func (i *Interface) WithPortSpeed(speed fpoc.E_IfEthernet_ETHERNET_SPEED) *Interface {
	i.oc.GetOrCreateEthernet().PortSpeed = speed
	return i
}

// WithDuplexMode sets duplex-mode on the specified interface.
func (i *Interface) WithDuplexMode(mode fpoc.E_IfEthernet_Ethernet_DuplexMode) *Interface {
	i.oc.GetOrCreateEthernet().DuplexMode = mode
	return i
}

// WithEnableFlowControl sets enable-flow-control on the specified interface.
func (i *Interface) WithEnableFlowControl(value bool) *Interface {
	i.oc.GetOrCreateEthernet().EnableFlowControl = ygot.Bool(value)
	return i
}

// AugmentDevice implements the device.Feature interface.
// This method augments the device OC with Interface feature.
// Use d.WithFeature(i) instead of calling this method directly.
func (i *Interface) AugmentDevice(d *fpoc.Device) error {
	if err := i.oc.Validate(); err != nil {
		return err
	}
	ioc := d.GetInterface(i.oc.GetName())
	if ioc == nil {
		return d.AppendInterface(&i.oc)
	}
	return ygot.MergeStructInto(ioc, &i.oc)
}

// Feature provides interface to augment Interface with additional features.
type Feature interface {
	// AugmentInterface augments Interface with additional features.
	AugmentInterface(oc *fpoc.Interface) error
}

// WithFeature augments Interface with provided feature.
func (i *Interface) WithFeature(f Feature) error {
	return f.AugmentInterface(&i.oc)
}
