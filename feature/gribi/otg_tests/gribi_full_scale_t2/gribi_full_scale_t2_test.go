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
func TestGRIBIFullScaleT2(t *testing.T) {
	params := cfgplugins.ScaleParams{
		// gRIBI & System parameters
		GRIBIBatchSize: 2_000,

		// Default VRF parameters
		NumDefaultNH:  1_000,
		NumDefaultNHG: 1_000,
		DefaultNHGLoadBalance: []cfgplugins.NHGLoadBalancingParams{
			{Pct: 40, NumNextHops: 8},
			{Pct: 40, NumNextHops: 16},
			{Pct: 15, NumNextHops: 32},
			{Pct: 5, NumNextHops: 64},
		},
		NumDefaultIPv4: 1_000,

		// Transit VRF parameters
		NumTransitNH:   4_000,
		NumTransitNHG:  2_000,
		NumTransitIPv4: 200_000,

		// Repair VRF parameters
		NumRepairIPv4: 200_000,
		NumRepairNHG:  1_000,
		PctNHG512:     70,

		// Encap / Decap VRF parameters
		NumEncapVRFs:       16,
		NumEncapIPv4PerVRF: 9_000,
		NumEncapIPv6PerVRF: 11_000,
		NumUniqueEncapNH:   16_000,
		NumEncapDefaultNHG: 4_000,
		PctEncap8NH:        75,
		PctEncap32NH:       20,

		// Decap VRF parameters
		NumDecapEntries:     50,
		DecapDestsSubsetPct: 100,

		// OTG / Port parameters
		NumPort2VLANs: 640,

		// Traffic parameters
		TrafficRateMpps: 30_000_000,
		TrafficDuration: 5 * time.Minute,
		TrafficLossTol:  5,
	}
	cfgplugins.RunFullScaleTest(t, params, *enablePacketCapture, *compactOTGFlows)
}
