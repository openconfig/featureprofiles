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

// Package sflow implements the Config Library for SFLOW feature profile.
package sflow

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Sflow struct to store OC attributes.
type Sflow struct {
	oc fpoc.Sampling_Sflow
}

// New returns a new Sflow object with the feature enabled.
func New() *Sflow {
	return &Sflow{
		oc: fpoc.Sampling_Sflow{
			Enabled: ygot.Bool(true),
		},
	}
}

// WithAgentIDIPv4 sets agent-id-ipv4.
func (s *Sflow) WithAgentIDIPv4(id string) *Sflow {
	s.oc.AgentIdIpv4 = ygot.String(id)
	return s
}

// WithAgentIDIPv6 sets agent-id-ipv6.
func (s *Sflow) WithAgentIDIPv6(id string) *Sflow {
	s.oc.AgentIdIpv6 = ygot.String(id)
	return s
}

// WithEgressSamplingRate sets egress-sampling-rate.
func (s *Sflow) WithEgressSamplingRate(rate int) *Sflow {
	s.oc.EgressSamplingRate = ygot.Uint32(uint32(rate))
	return s
}

// WithIngressSamplingRate sets ingress-sampling-rate.
func (s *Sflow) WithIngressSamplingRate(rate int) *Sflow {
	s.oc.IngressSamplingRate = ygot.Uint32(uint32(rate))
	return s
}

// WithSampleSize sets sample-size.
func (s *Sflow) WithSampleSize(size int) *Sflow {
	s.oc.SampleSize = ygot.Uint16(uint16(size))
	return s
}

// AugmentDevice implements the device.Feature interface.
// This method augments the device OC with Sflow feature.
// Use d.WithFeature(l) instead of calling this method directly.
func (s *Sflow) AugmentDevice(d *fpoc.Device) error {
	if err := s.oc.Validate(); err != nil {
		return err
	}
	sampling := d.GetOrCreateSampling()
	if sampling.Sflow == nil {
		sampling.Sflow = &s.oc
		return nil
	}
	return ygot.MergeStructInto(sampling.Sflow, &s.oc)
}

// Feature provides interface to augment Sflow with additional features.
type Feature interface {
	// AugmentSflow augments Sflow with additional features.
	AugmentSflow(oc *fpoc.Sampling_Sflow) error
}

// WithFeature augments Sflow with provided feature.
func (s *Sflow) WithFeature(f Feature) error {
	return f.AugmentSflow(&s.oc)
}
