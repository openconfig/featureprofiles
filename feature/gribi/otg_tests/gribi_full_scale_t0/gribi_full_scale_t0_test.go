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

// Package gribi_full_scale_t0_test implements TE-14.5: gRIBI Scaling - full scale setup, target T0.
//
// Scale constants for T0:
//
//	pctNHG512=80%, numRepairNHG=500, numEncapDefaultNHG=2.5K, numUniqueEncapNH=10K
//
// Test structure (per README TE-14.5):
//
//	TestGRIBIFullScaleT0 — configures DUT+ATE once, programs gRIBI once, then runs
//	                        both fixed-size (64B) and IMIX traffic profiles as sub-tests,
//	                        each executing all five traffic scenarios simultaneously in a
//	                        single 30 Mpps traffic pass and validates:
//	  1. Zero packet loss per flow.
//	  2. Outer-src IP correctness per scenario (encap → src111, repaired → src222, …).
//	  3. DSCP preservation end-to-end.
//	  4. Encap presence/absence (inner vs outer header inspection via OTG capture).
package gribifullscalet0_test

import (
	"flag"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
)

// ============================================================
// Constants — T0-specific scale parameters (TE-14.3)
// ============================================================

const (
	// pctNHG512T0 is the percentage of Default VRF NHGs with 1/512 granularity.
	pctNHG512T0 = 80

	// numRepairNHGT0 is the number of NHGs in REPAIR_VRF for T0.
	numRepairNHGT0 = 500

	// numEncapDefaultNHGT0 is the T3 scale target: NHGs in the default VRF
	// that back encap VRF entries.
	numEncapDefaultNHGT0 = 2_500

	// numUniqueEncapNHT0 is the T4 scale target: total unique encap NHs.
	numUniqueEncapNHT0 = 10_000
)

var (
	enablePacketCapture = flag.Bool("enable_packet_capture", false, "Enable packet capture and deep packet inspection validation.")
	compactOTGFlows     = flag.Bool("compact_otg_flows", true, "Compact OTG flows to reduce the number of flows due to OTG port limits.")
)

// ============================================================
// TestMain
// ============================================================

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// ============================================================
// Test
// ============================================================

// TestGRIBIFullScaleT0 validates TE-14.5 by running both fixed-size (64B) and
// IMIX traffic profiles using a table-driven approach. It performs full DUT
// setup once and executes all five traffic scenarios in a single 30 Mpps
// traffic pass per sub-test.
func TestGRIBIFullScaleT0(t *testing.T) {
	params := cfgplugins.ScaleParams{
		PctNHG512:          pctNHG512T0,
		NumRepairNHG:       numRepairNHGT0,
		NumEncapDefaultNHG: numEncapDefaultNHGT0,
		NumUniqueEncapNH:   numUniqueEncapNHT0,

		NumDefaultNH:       1_000,
		NumDefaultNHG:      1_000,
		NumDefaultIPv4:     1_000,
		NumTransitNHD1:     1536,
		NumTransitNHD2:     1536,
		NumTransitNHGE1:    768,
		NumTransitNHGE2:    768,
		NumTransitIPv4:     12_600,
		NumRepairIPv4:      12_600,
		NumEncapVRFs:       5,
		NumEncapIPv4PerVRF: 10_000,
		NumEncapIPv6PerVRF: 10_000,
		NumDecapEntries:    48,
		TrafficDuration:    5 * time.Minute,
		TrafficLossTol:     5,
		TrafficRateMpps:    30_000_000,

		NumPort1VLANs:       1,
		NumPort2VLANs:       640,
		PctEncap8NH:         75,
		PctEncap32NH:        20,
		DecapDestsSubsetPct: 10,
		GRIBIBatchSize:      2_000,
	}
	cfgplugins.RunFullScaleTest(t, params, *enablePacketCapture, *compactOTGFlows)
}
