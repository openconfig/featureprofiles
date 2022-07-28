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

// Package aggregate implements the Config Library for aggregate interface feature profile.
package aggregate

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Aggregate struct to store OC attributes.
type Aggregate struct {
	aggioc fpoc.Interface
	lioc   fpoc.Lacp_Interface
	goc    fpoc.Lacp
	mioc   map[string]*fpoc.Interface
}

// New returns a new Aggregate object.
func New(aggIntfName string, lagType fpoc.E_IfAggregate_AggregationType, interval fpoc.E_Lacp_LacpPeriodType) *Aggregate {
	return &Aggregate{
		aggioc: fpoc.Interface{
			Name: ygot.String(aggIntfName),
			Aggregation: &fpoc.Interface_Aggregation{
				LagType: lagType,
			},
		},
		lioc: fpoc.Lacp_Interface{
			Name:     ygot.String(aggIntfName),
			Interval: interval,
		},
		mioc: map[string]*fpoc.Interface{},
	}
}

// WithMinLinks sets min-links on the specified aggregate interface.
func (a *Aggregate) WithMinLinks(minLinks int) *Aggregate {
	a.aggioc.GetOrCreateAggregation().MinLinks = ygot.Uint16(uint16(minLinks))
	return a
}

// WithLACPMode sets lacp-mode on the specified aggregate interface.
func (a *Aggregate) WithLACPMode(mode fpoc.E_Lacp_LacpActivityType) *Aggregate {
	a.lioc.LacpMode = mode
	return a
}

// WithSystemIDMAC sets system-id-mac on the specified aggregate interface.
func (a *Aggregate) WithSystemIDMAC(mac string) *Aggregate {
	a.lioc.SystemIdMac = ygot.String(mac)
	return a
}

// WithInterfaceSystemPriority sets system-priority on the specified aggregate interface.
func (a *Aggregate) WithInterfaceSystemPriority(pri int) *Aggregate {
	a.lioc.SystemPriority = ygot.Uint16(uint16(pri))
	return a
}

// WithGlobalSystemPriority sets system-priority globally for all aggregate interfaces.
func (a *Aggregate) WithGlobalSystemPriority(pri int) *Aggregate {
	a.goc.SystemPriority = ygot.Uint16(uint16(pri))
	return a
}

// AddMember adds member interface to aggregate.
func (a *Aggregate) AddMember(memberIntfName string) *Aggregate {
	mioc, ok := a.mioc[memberIntfName]
	if !ok {
		mioc = &fpoc.Interface{}
		a.mioc[memberIntfName] = mioc
	}
	mioc.Name = ygot.String(memberIntfName)
	mioc.GetOrCreateEthernet().AggregateId = ygot.String(a.aggioc.GetName())
	return a
}

// AugmentDevice implements the device.Feature interface.
// This method augments the device OC with Aggregate interfaces.
// Use d.WithFeature(a) instead of calling this method directly.
func (a *Aggregate) AugmentDevice(d *fpoc.Device) error {
	// Augment device with global LACP.
	if err := a.goc.Validate(); err != nil {
		return err
	}
	goc := d.GetLacp()
	if goc == nil {
		d.Lacp = &a.goc
		goc = &a.goc
	} else {
		if err := ygot.MergeStructInto(goc, &a.goc); err != nil {
			return err
		}
	}

	// Augment device with the LACP interface.
	if err := a.lioc.Validate(); err != nil {
		return err
	}
	lioc := goc.GetInterface(a.lioc.GetName())
	if lioc == nil {
		if err := goc.AppendInterface(&a.lioc); err != nil {
			return err
		}
	} else {
		if err := ygot.MergeStructInto(lioc, &a.lioc); err != nil {
			return err
		}
	}

	// Augment device with the aggregate interface.
	if err := a.aggioc.Validate(); err != nil {
		return err
	}
	aggioc := d.GetInterface(a.aggioc.GetName())
	if aggioc == nil {
		if err := d.AppendInterface(&a.aggioc); err != nil {
			return err
		}
	} else {
		if err := ygot.MergeStructInto(aggioc, &a.aggioc); err != nil {
			return err
		}
	}

	// Augment device with the member interfaces.
	for memberIntfName, mioc := range a.mioc {
		if err := mioc.Validate(); err != nil {
			return err
		}
		dmioc := d.GetInterface(memberIntfName)
		if dmioc == nil {
			if err := d.AppendInterface(mioc); err != nil {
				return err
			}
		} else {
			if err := ygot.MergeStructInto(dmioc, mioc); err != nil {
				return err
			}
		}
	}
	return nil
}
