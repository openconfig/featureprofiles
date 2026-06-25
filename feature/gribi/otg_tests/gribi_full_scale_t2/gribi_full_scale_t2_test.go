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
	"flag"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
	params := cfgplugins.ScaleParams{
		PctNHG512:          pctNHG512T2,
		NumRepairNHG:       numRepairNHGT2,
		NumEncapDefaultNHG: numEncapDefaultNHGT2,
		NumUniqueEncapNH:   numUniqueEncapNHT2,

		NumDefaultNH:       1_000,
		NumDefaultNHG:      1_000,
		NumDefaultIPv4:     1_000,
		NumTransitNHD1:     1536,
		NumTransitNHD2:     1536,
		NumTransitNHGE1:    768,
		NumTransitNHGE2:    768,
		NumTransitIPv4:     200_000,
		NumRepairIPv4:      200_000,
		NumEncapVRFs:       16,
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
