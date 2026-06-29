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

// Package gribi_full_scale_down_test implements TE-14.6: gRIBI Scaling - all scenarios but with minimal scaling parameters.
// This test is helpful to validate the correctness of the gRIBI programming and the OTG traffic
// validation logic.
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
package gribifullscaledown_test

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
	// pctNHG512 is the percentage of Default VRF NHGs with 1/512 granularity.
	pctNHG512 = 80

	// numRepairNHG is the number of NHGs in REPAIR_VRF for T0.
	numRepairNHG = 1

	// numEncapDefaultNHG is the T3 scale target: NHGs in the default VRF
	// that back encap VRF entries.
	numEncapDefaultNHG = 1

	// numUniqueEncapNH is the T4 scale target: total unique encap NHs.
	numUniqueEncapNH = 1
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
func TestGRIBIFullScaleDown(t *testing.T) {
	params := cfgplugins.ScaleParams{
		PctNHG512:          pctNHG512,
		NumRepairNHG:       numRepairNHG,
		NumEncapDefaultNHG: numEncapDefaultNHG,
		NumUniqueEncapNH:   numUniqueEncapNH,

		NumDefaultNH:       1,
		NumDefaultNHG:      1,
		NumDefaultIPv4:     1,
		NumTransitNHD1:     1,
		NumTransitNHD2:     1,
		NumTransitNHGE1:    1,
		NumTransitNHGE2:    1,
		NumTransitIPv4:     1,
		NumRepairIPv4:      1,
		NumEncapVRFs:       1,
		NumEncapIPv4PerVRF: 1,
		NumEncapIPv6PerVRF: 1,
		NumDecapEntries:    1,
		TrafficDuration:    1 * time.Minute,
		TrafficLossTol:     5,
		TrafficRateMpps:    1_000,

		NumPort1VLANs:       1,
		NumPort2VLANs:       1,
		PctEncap8NH:         75,
		PctEncap32NH:        20,
		DecapDestsSubsetPct: 10,
		GRIBIBatchSize:      256,
	}
	cfgplugins.RunFullScaleTest(t, params, *enablePacketCapture, *compactOTGFlows)
}
