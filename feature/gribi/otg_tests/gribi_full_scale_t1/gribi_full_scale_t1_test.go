// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gribi_full_scale_t1_test implements TE-14.3: gRIBI Scaling - full scale setup, target T1.
//
// Scale constants for T1:
//
//	pctNHG512=80%, numRepairNHG=1K, numEncapDefaultNHG=4K, numUniqueEncapNH=16K
//
// Test structure (per README TE-14.3):
//
//	TestGRIBIFullScaleT1 — configures DUT+ATE once, programs gRIBI once, then runs
//	                        both fixed-size (64B) and IMIX traffic profiles as sub-tests,
//	                        each executing all five traffic scenarios simultaneously in a
//	                        single 30 Mpps traffic pass and validates:
//	  1. Zero packet loss per flow.
//	  2. Outer-src IP correctness per scenario (encap → src111, repaired → src222, …).
//	  3. DSCP preservation end-to-end.
//	  4. Encap presence/absence (inner vs outer header inspection via OTG capture).
package gribifullscalet1_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

// ============================================================
// Constants — T1-specific scale parameters (TE-14.3)
// ============================================================

const (
	// pctNHG512T1 is the percentage of Default VRF NHGs with 1/512 granularity.
	// T1: 80%, T2: 20%.
	pctNHG512T1 = 80

	// numRepairNHGT1 is the number of NHGs in REPAIR_VRF for T1.
	// T1: 1K, T2: 2K.
	numRepairNHGT1 = 1_000

	// numEncapDefaultNHGT1 is the T3 scale target: NHGs in the default VRF
	// that back encap VRF entries.
	// T1: 4K, T2: 8K.
	numEncapDefaultNHGT1 = 4_000

	// numUniqueEncapNHT1 is the T4 scale target: total unique encap NHs.
	// T1: 16K, T2: 32K.
	numUniqueEncapNHT1 = 16_000
)

// ============================================================
// Types
// ============================================================

// trafficTestCase is a table-driven entry for the two traffic profiles.
// Re-declared locally to keep this package self-contained; it mirrors
// cfgplugins.TrafficTestCase.
type trafficTestCase = cfgplugins.TrafficTestCase

// ============================================================
// TestMain
// ============================================================

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// ============================================================
// Test
// ============================================================

// TestGRIBIFullScaleT1 validates TE-14.3 by running both fixed-size (64B) and
// IMIX traffic profiles using a table-driven approach. It performs full DUT
// setup once and executes all five traffic scenarios in a single 30 Mpps
// traffic pass per sub-test.
func TestGRIBIFullScaleT1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	defaultVRF := deviations.DefaultNetworkInstance(dut)
	ctx := context.Background()
	t.Log("Configuring DUT interfaces, VRFs, and VRF-selection policy")
	cfgplugins.ConfigureDUT(t, dut)

	t.Log("Configuring ATE topology")
	ateConfig, interfaceNamesList := cfgplugins.ConfigureOTG(t, ate, dut)
	ate.OTG().PushConfig(t, ateConfig)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	t.Run("Configure and validate FIB_PROGRAMMED, Hierarchical route structure", func(t *testing.T) {
		// DEFAULT VRF
		t.Log("Default VRF entries (A/B/C)")
		defaultPrefixes := cfgplugins.BuildDefaultVRF(t, dut, ctx, defaultVRF, pctNHG512T1)

		// Static Groups
		t.Log("Static groups (S1/S2)")
		s1NHG, s2NHG := cfgplugins.BuildStaticGroups(t, dut, ctx, defaultVRF)

		// Repair VRF
		t.Log("Repair VRF (F)")
		cfgplugins.BuildRepairVRF(t, dut, ctx, defaultVRF, s2NHG, numRepairNHGT1)

		// Transit VRFs
		t.Log("Transit VRFs (D/E)")
		cfgplugins.BuildTransitVRFs(t, dut, ctx, defaultVRF, defaultPrefixes, s1NHG, s2NHG)

		// Encap/Decap VRFs
		t.Log("Encap/Decap VRFs (T3/T4)")
		cfgplugins.BuildEncapDecapVRFs(t, dut, ctx, defaultVRF, numEncapDefaultNHGT1, numUniqueEncapNHT1)
	})

	testCases := []trafficTestCase{
		{Name: "FixedSize_64B", UseIMIX: false},
		{Name: "IMIX_Profile", UseIMIX: true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.UseIMIX {
				t.Log("Running IMIX traffic — all 5 scenarios, 30 Mpps aggregate")
			} else {
				t.Log("Running fixed-size (64B) traffic — all 5 scenarios, 30 Mpps aggregate")
			}
			cfgplugins.RunEndToEndTrafficValidation(t, ate, dut, ateConfig, tc.UseIMIX)
		})
	}
}
