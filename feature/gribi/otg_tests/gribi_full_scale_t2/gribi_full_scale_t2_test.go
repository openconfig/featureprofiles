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

// Package gribi_full_scale_t2_test implements TE-14.4: gRIBI Scaling - full scale setup, target T2.
//
// Scale constants for T2:
//
//	pctNHG512=70%, numRepairNHG=2K, numEncapDefaultNHG=8K, numUniqueEncapNH=32K
//
// Test structure (per README TE-14.4):
//
//	TestGRIBIFullScaleT2 — configures DUT+ATE once, programs gRIBI once, then runs
//	                        both fixed-size (64B) and IMIX traffic profiles as sub-tests,
//	                        each executing all five traffic scenarios simultaneously in a
//	                        single 30 Mpps traffic pass and validates:
//	  1. Zero packet loss per flow.
//	  2. Outer-src IP correctness per scenario (encap → src111, repaired → src222, …).
//	  3. DSCP preservation end-to-end.
//	  4. Encap presence/absence (inner vs outer header inspection via OTG capture).
package gribifullscalet2_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

// ============================================================
// Constants — T2-specific scale parameters (TE-14.4)
// ============================================================

const (
	// pctNHG512T2 is the percentage of Default VRF NHGs with 1/512 granularity.
	// T1: 80%, T2: 70%.
	pctNHG512T2 = 70

	// numRepairNHGT2 is the number of NHGs in REPAIR_VRF for T2.
	// T1: 1K, T2: 2K.
	numRepairNHGT2 = 2_000

	// numEncapDefaultNHGT2 is the T3 scale target: NHGs in the default VRF
	// that back encap VRF entries.
	// T1: 4K, T2: 8K.
	numEncapDefaultNHGT2 = 8_000

	// numUniqueEncapNHT2 is the T4 scale target: total unique encap NHs.
	// T1: 16K, T2: 32K.
	numUniqueEncapNHT2 = 32_000
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

// TestGRIBIFullScaleT2 validates TE-14.4 by running both fixed-size (64B) and
// IMIX traffic profiles using a table-driven approach. It performs full DUT
// setup once and executes all five traffic scenarios in a single 30 Mpps
// traffic pass per sub-test.
//
// gRIBI programming is performed incrementally: each VRF builder creates its
// own persistent gRIBI client, pushes its entries, validates the FIB, then
// closes the connection. Entries remain installed on the DUT (Persistence:
// true) until the single cleanup client issues a FlushAll at test teardown.
func TestGRIBIFullScaleT2(t *testing.T) {
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
	// Limiting it to 100 since checking ARP for 1024 interfaces takes long time
	ifs := interfaceNamesList
	if len(ifs) >= 100 {
		ifs = ifs[:100]
	}
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: ifs})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: ifs})

	t.Run("Configure and validate FIB_PROGRAMMED, Hierarchical route structure", func(t *testing.T) {
		// DEFAULT VRF
		t.Log("Default VRF entries (A/B/C)")
		defaultPrefixes := cfgplugins.BuildDefaultVRF(t, dut, ctx, defaultVRF, pctNHG512T2)

		// Static Groups
		t.Log("Static groups (S1/S2)")
		s1NHG, s2NHG := cfgplugins.BuildStaticGroups(t, dut, ctx, defaultVRF)

		// Repair VRF
		t.Log("Repair VRF (F)")
		cfgplugins.BuildRepairVRF(t, dut, ctx, defaultVRF, s2NHG, numRepairNHGT2)

		// Transit VRFs
		t.Log("Transit VRFs (D/E)")
		cfgplugins.BuildTransitVRFs(t, dut, ctx, defaultVRF, defaultPrefixes, s1NHG, s2NHG)

		// Encap/Decap VRFs
		t.Log("Encap/Decap VRFs (T3/T4)")
		cfgplugins.BuildEncapDecapVRFs(t, dut, ctx, defaultVRF, numEncapDefaultNHGT2, numUniqueEncapNHT2)
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
