// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traceroute_packetin_with_vrf_selection_test

import (
	"reflect"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

func TestUDPFlowEntropy(t *testing.T) {
	outer, inner := gosnappi.NewFlowUdp(), gosnappi.NewFlowUdp()
	configureUDPFlowEntropy(outer, 1, 2)
	configureUDPFlowEntropy(inner, 3, 4)
	for _, udp := range []gosnappi.FlowUdp{outer, inner} {
		if _, err := udp.Marshal().ToProto(); err != nil {
			t.Fatalf("Invalid OTG UDP header: %v", err)
		}
	}
	sequences := [][]uint32{
		outer.SrcPort().Values(), outer.DstPort().Values(),
		inner.SrcPort().Values(), inner.DstPort().Values(),
	}
	for i, values := range sequences {
		if len(values) != traceroutePacketsPerFlow {
			t.Fatalf("Field %d has %d values, want %d", i, len(values), traceroutePacketsPerFlow)
		}
		seen := make(map[uint32]bool)
		for _, port := range values {
			if port < 1024 || port > 65535 {
				t.Fatalf("Field %d has out-of-range port %d", i, port)
			}
			if seen[port] {
				t.Fatalf("Field %d repeats port %d within the burst", i, port)
			}
			seen[port] = true
		}
		if !reflect.DeepEqual(values, udpPortValues(int64(i+1))) {
			t.Errorf("Field %d is not reproducible with the same seed", i)
		}
		for j := 0; j < i; j++ {
			if reflect.DeepEqual(values, sequences[j]) {
				t.Errorf("Fields %d and %d have identical entropy sequences", i, j)
			}
		}
	}
}
