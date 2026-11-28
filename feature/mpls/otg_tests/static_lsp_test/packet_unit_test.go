// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package static_lsp_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
)

// Run without reserving hardware:
// go test packet_test.go packet_unit_test.go
func TestMPLSoGUEPacketSizes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inner    *otgconfighelpers.Flow
		minimum  uint32
		ipHeader string
	}{
		{"IPv4", innerIPv4Flow(), 120, "ipv4"},
		{"IPv6", innerIPv6Flow(), 140, "ipv6"},
		{"IPv4 DSCP sweep", innerIPv4PreserveSweepFlow(), 120, "ipv4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := gosnappi.NewConfig()
			createMPLSoGUEFlow(cfg, "flow", "source.IPv4", "02:00:00:00:00:01", 99991, tc.inner, "destination.IP")
			flow := cfg.Flows().Items()[0]
			if _, err := cfg.Marshal().ToJson(); err != nil {
				t.Fatalf("Invalid OTG configuration: %v", err)
			}
			var weight, shortWeight, jumboWeight float32
			for _, pair := range flow.Size().WeightPairs().Custom().Items() {
				if pair.Size() < tc.minimum {
					t.Errorf("Frame size %d cannot accommodate encapsulation/tracking minimum %d", pair.Size(), tc.minimum)
				}
				weight += pair.Weight()
				if pair.Size() < 256 {
					shortWeight += pair.Weight()
				}
				if pair.Size() == 9000 {
					jumboWeight += pair.Weight()
				}
			}
			if weight != 100 || shortWeight != 40 || jumboWeight != 2 {
				t.Errorf("IMIX weights: total=%v short=%v jumbo=%v, want 100/40/2", weight, shortWeight, jumboWeight)
			}
			var headers []string
			for _, header := range flow.Packet().Items() {
				headers = append(headers, fmt.Sprint(header.Choice()))
			}
			want := []string{"ethernet", "ipv4", "udp", "mpls", tc.ipHeader, "tcp"}
			if !slices.Equal(headers, want) {
				t.Errorf("Packet headers = %v, want %v", headers, want)
			}
			if got := flow.Duration().FixedPackets().Packets(); got != trafficPackets {
				t.Errorf("Packet count = %d, want %d", got, trafficPackets)
			}
		})
	}
}
