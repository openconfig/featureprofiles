// Copyright 2026 Google LLC
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

package plq

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

func seconds(s int64) *durationpb.Duration {
	return &durationpb.Duration{Seconds: s}
}

// capabilities returns a CapabilitiesResponse with the given generator minimum
// setup and teardown durations and reflector loopback capabilities.
func capabilities(genSetup, genTeardown int64, asic *plqpb.AsicLoopbackCapabilities, pmd *plqpb.PmdLoopbackCapabilities) *plqpb.CapabilitiesResponse {
	return &plqpb.CapabilitiesResponse{
		Generator: &plqpb.GeneratorCapabilities{
			PacketGenerator: &plqpb.PacketGeneratorCapabilities{
				MinSetupDuration:    seconds(genSetup),
				MinTeardownDuration: seconds(genTeardown),
			},
		},
		Reflector: &plqpb.ReflectorCapabilities{
			AsicLoopback: asic,
			PmdLoopback:  pmd,
		},
	}
}

// defaultTiming returns the timing expected when every device minimum is below
// the gNOI-2.1 minimums.
func defaultTiming() *Timing {
	return &Timing{
		GeneratorPreSyncDuration:  10 * time.Second,
		GeneratorSetupDuration:    30 * time.Second,
		GeneratorPostSyncDuration: 0,
		GeneratorTeardownDuration: 30 * time.Second,
		ReflectorPreSyncDuration:  0,
		ReflectorSetupDuration:    40 * time.Second,
		ReflectorPostSyncDuration: 30 * time.Second,
		ReflectorTeardownDuration: 10 * time.Second,
		TestDuration:              180 * time.Second,
	}
}

func TestComputeTiming(t *testing.T) {
	asic := &plqpb.AsicLoopbackCapabilities{MinSetupDuration: seconds(5), MinTeardownDuration: seconds(5)}
	pmd := &plqpb.PmdLoopbackCapabilities{MinSetupDuration: seconds(2), MinTeardownDuration: seconds(3)}

	largeMinimums := defaultTiming()
	largeMinimums.GeneratorSetupDuration = 45 * time.Second
	largeMinimums.GeneratorTeardownDuration = 50 * time.Second
	largeMinimums.ReflectorSetupDuration = 60 * time.Second
	largeMinimums.ReflectorTeardownDuration = 20 * time.Second

	fractionalCaps := capabilities(5, 5, asic, nil)
	fractionalCaps.GetGenerator().GetPacketGenerator().MinSetupDuration = &durationpb.Duration{Seconds: 45, Nanos: 5e8}
	fractionalMinimum := defaultTiming()
	fractionalMinimum.GeneratorSetupDuration = 45500 * time.Millisecond

	tests := []struct {
		desc     string
		caps     *plqpb.CapabilitiesResponse
		want     *Timing
		wantMode ReflectorMode
		wantErr  bool
	}{
		{
			desc:     "small device minimums use the gNOI-2.1 minimums",
			caps:     capabilities(5, 5, asic, nil),
			want:     defaultTiming(),
			wantMode: AsicLoopback,
		},
		{
			desc: "larger device minimums are used",
			caps: capabilities(45, 50, &plqpb.AsicLoopbackCapabilities{
				MinSetupDuration:    seconds(60),
				MinTeardownDuration: seconds(20),
			}, nil),
			want:     largeMinimums,
			wantMode: AsicLoopback,
		},
		{
			desc:     "fractional device minimums are not truncated",
			caps:     fractionalCaps,
			want:     fractionalMinimum,
			wantMode: AsicLoopback,
		},
		{
			desc:     "ASIC loopback is preferred when both modes are supported",
			caps:     capabilities(5, 5, asic, pmd),
			want:     defaultTiming(),
			wantMode: AsicLoopback,
		},
		{
			desc:     "PMD loopback is used without ASIC loopback",
			caps:     capabilities(5, 5, nil, pmd),
			want:     defaultTiming(),
			wantMode: PmdLoopback,
		},
		{
			desc: "PMD loopback is used when ASIC loopback has no teardown duration",
			caps: capabilities(5, 5, &plqpb.AsicLoopbackCapabilities{
				MinSetupDuration: seconds(5),
			}, pmd),
			want:     defaultTiming(),
			wantMode: PmdLoopback,
		},
		{
			desc:    "error without a supported reflector mode",
			caps:    capabilities(5, 5, nil, nil),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, gotMode, err := computeTiming(tc.caps)
			if (err != nil) != tc.wantErr {
				t.Fatalf("computeTiming() error: got %v, want error: %t", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("computeTiming() timing diff (-want +got):\n%s", diff)
			}
			if gotMode != tc.wantMode {
				t.Errorf("computeTiming() mode: got %v, want %v", gotMode, tc.wantMode)
			}
		})
	}
}

func TestTimingWindows(t *testing.T) {
	tm := defaultTiming()
	// Generator: 10s pre-sync + 30s setup; reflector: 0s pre-sync + 40s setup.
	if got, want := tm.SetupTime(), 50*time.Second; got != want {
		t.Errorf("SetupTime(): got %v, want %v", got, want)
	}
	// Generator: 10+30+180+0+30 = 250s; reflector: 0+40+180+30+10 = 260s.
	if got, want := tm.MaxCompletionTime(), 260*time.Second; got != want {
		t.Errorf("MaxCompletionTime(): got %v, want %v", got, want)
	}
}

func TestDiscrepancyPct(t *testing.T) {
	tests := []struct {
		desc   string
		a, b   uint64
		want   float64
		wantOK bool
	}{
		{desc: "equal counts", a: 1000, b: 1000, want: 0, wantOK: true},
		{desc: "one packet in a million", a: 1000000, b: 999999, want: 1.0000005e-4, wantOK: true},
		{desc: "one count is 0", a: 10, b: 0, want: 200, wantOK: true},
		{desc: "both counts are 0", a: 0, b: 0, wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, ok := discrepancyPct(tc.a, tc.b)
			if ok != tc.wantOK {
				t.Fatalf("discrepancyPct(%d, %d) ok: got %t, want %t", tc.a, tc.b, ok, tc.wantOK)
			}
			if math.Abs(got-tc.want) > 1e-12 {
				t.Errorf("discrepancyPct(%d, %d): got %g, want %g", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIDsAndPorts(t *testing.T) {
	pairs := []LinkPair{
		{GeneratorPort: "p1", ReflectorPort: "p2", GeneratorID: "g1", ReflectorID: "r1"},
		{GeneratorPort: "p3", ReflectorPort: "p4", GeneratorID: "g2", ReflectorID: "r2"},
	}
	if diff := cmp.Diff([]string{"g1", "r1", "g2", "r2"}, IDs(pairs)); diff != "" {
		t.Errorf("IDs() diff (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"p1", "p2", "p3", "p4"}, Ports(pairs)); diff != "" {
		t.Errorf("Ports() diff (-want +got):\n%s", diff)
	}
}

func TestPairConfigs(t *testing.T) {
	pair := LinkPair{GeneratorPort: "p1", ReflectorPort: "p2", GeneratorID: "g1", ReflectorID: "r1", PacketRate: 138888, PacketSize: 8184}
	tests := []struct {
		mode    ReflectorMode
		wantPMD bool
	}{
		{mode: AsicLoopback, wantPMD: false},
		{mode: PmdLoopback, wantPMD: true},
	}
	for _, tc := range tests {
		t.Run(tc.mode.String(), func(t *testing.T) {
			cfgs := pairConfigs(pair, defaultTiming(), tc.mode)
			if len(cfgs) != 2 {
				t.Fatalf("pairConfigs() returned %d configurations, want 2", len(cfgs))
			}
			gen, ref := cfgs[0], cfgs[1]
			if got := gen.GetPacketGenerator(); got.GetPacketRate() != 138888 || got.GetPacketSize() != 8184 {
				t.Errorf("generator PacketGeneratorConfiguration: got %v, want packet_rate 138888 and packet_size 8184", got)
			}
			if got, want := gen.GetRpc().GetPreSyncDuration().AsDuration(), 10*time.Second; got != want {
				t.Errorf("generator pre_sync_duration: got %v, want %v", got, want)
			}
			if got, want := ref.GetRpc().GetPostSyncDuration().AsDuration(), 30*time.Second; got != want {
				t.Errorf("reflector post_sync_duration: got %v, want %v", got, want)
			}
			if gotPMD := ref.GetPmdLoopback() != nil; gotPMD != tc.wantPMD {
				t.Errorf("reflector uses PMD loopback: got %t, want %t", gotPMD, tc.wantPMD)
			}
			if gotASIC := ref.GetAsicLoopback() != nil; gotASIC == tc.wantPMD {
				t.Errorf("reflector uses ASIC loopback: got %t, want %t", gotASIC, !tc.wantPMD)
			}
		})
	}
}
