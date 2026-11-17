// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plq

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	rpcpb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

func TestSpeedToBPS(t *testing.T) {
	tests := []struct {
		name  string
		speed ondatra.Speed
		want  uint64
	}{
		{name: "100G", speed: ondatra.Speed100Gb, want: 100_000_000_000},
		{name: "400G", speed: ondatra.Speed400Gb, want: 400_000_000_000},
		{name: "800G", speed: ondatra.Speed800Gb, want: 800_000_000_000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SpeedToBPS(tc.speed); got != tc.want {
				t.Errorf("SpeedToBPS(%v) = %d, want %d", tc.speed, got, tc.want)
			}
		})
	}
}

func TestCalculateRateAndSize(t *testing.T) {
	highCap := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 800_000_000_000,
		MaxPps: 100_000_000,
		MinMtu: 64,
		MaxMtu: 8184,
	}
	cisco8202 := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 62_976_088_000,
		MaxPps: 13_643_000,
		MinMtu: 64,
		MaxMtu: 577,
	}

	tests := []struct {
		name            string
		portSpeedBPS    uint64
		caps            *plqpb.PacketGeneratorCapabilities
		concurrentLinks int
		want            TrafficParams
		wantErr         bool
	}{
		{
			name:            "high_cap_100g_single",
			portSpeedBPS:    100_000_000_000,
			caps:            highCap,
			concurrentLinks: 1,
			want: TrafficParams{
				PacketRate: 833_333,
				PacketSize: 1500,
				TargetBPS:  833_333 * 1500 * 8,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "high_cap_400g_single",
			portSpeedBPS:    400_000_000_000,
			caps:            highCap,
			concurrentLinks: 1,
			want: TrafficParams{
				PacketRate: 3_333_333,
				PacketSize: 1500,
				TargetBPS:  3_333_333 * 1500 * 8,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "high_cap_800g_single",
			portSpeedBPS:    800_000_000_000,
			caps:            highCap,
			concurrentLinks: 1,
			want: TrafficParams{
				PacketRate: 6_666_666,
				PacketSize: 1500,
				TargetBPS:  6_666_666 * 1500 * 8,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "cisco8202_100g_4_concurrent",
			portSpeedBPS:    100_000_000_000,
			caps:            cisco8202,
			concurrentLinks: 4,
			want: TrafficParams{
				PacketRate: 2_166_377,
				PacketSize: 577,
				TargetBPS:  2_166_377 * 577 * 8,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "cisco8202_800g_capped_by_max_pps",
			portSpeedBPS:    800_000_000_000,
			caps:            cisco8202,
			concurrentLinks: 1,
			want: TrafficParams{
				PacketRate: 13_643_000,
				PacketSize: 577,
				TargetBPS:  62_976_088_000,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "cisco8202_400g_4_concurrent_shares_budget",
			portSpeedBPS:    400_000_000_000,
			caps:            cisco8202,
			concurrentLinks: 4,
			want: TrafficParams{
				PacketRate: 3_410_750,
				PacketSize: 577,
				TargetBPS:  3_410_750 * 577 * 8,
				Duration:   60 * time.Second,
			},
		},
		{
			name:            "zero_port_speed_fails",
			portSpeedBPS:    0,
			caps:            highCap,
			concurrentLinks: 1,
			wantErr:         true,
		},
		{
			name:            "zero_concurrent_links_fails",
			portSpeedBPS:    100_000_000_000,
			caps:            highCap,
			concurrentLinks: 0,
			wantErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateRateAndSize(tc.portSpeedBPS, tc.caps, tc.concurrentLinks, 0)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CalculateRateAndSize() error = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("CalculateRateAndSize() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCalculateMaxPPSParams(t *testing.T) {
	cisco8202 := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 62_976_088_000,
		MaxPps: 13_643_000,
		MinMtu: 64,
		MaxMtu: 577,
	}
	cisco8808 := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 188_928_264_000,
		MaxPps: 40_929_000,
		MinMtu: 64,
		MaxMtu: 577,
	}
	highCap := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 800_000_000_000,
		MaxPps: 100_000_000,
		MinMtu: 64,
		MaxMtu: 8184,
	}
	extremePPS := &plqpb.PacketGeneratorCapabilities{
		MaxBps: 800_000_000_000,
		MaxPps: 500_000_000,
		MinMtu: 64,
		MaxMtu: 8184,
	}

	tests := []struct {
		name         string
		portSpeedBPS uint64
		caps         *plqpb.PacketGeneratorCapabilities
		wantRate     uint64
		wantSize     uint32
	}{
		{
			name:         "cisco8202_100g_full_13.6Mpps_at_577B",
			portSpeedBPS: 100_000_000_000,
			caps:         cisco8202,
			wantRate:     13_643_000,
			wantSize:     577,
		},
		{
			name:         "cisco8808_100g_shrinks_packet_size_to_fit_100g_wire",
			portSpeedBPS: 100_000_000_000,
			caps:         cisco8808,
			wantRate:     40_929_000,
			wantSize:     270,
		},
		{
			name:         "cisco8808_400g_full_max_mtu_and_pps",
			portSpeedBPS: 400_000_000_000,
			caps:         cisco8808,
			wantRate:     40_929_000,
			wantSize:     577,
		},
		{
			name:         "high_cap_100mpps_on_100g",
			portSpeedBPS: 100_000_000_000,
			caps:         highCap,
			wantRate:     100_000_000,
			wantSize:     98,
		},
		{
			name:         "high_cap_100mpps_on_400g",
			portSpeedBPS: 400_000_000_000,
			caps:         highCap,
			wantRate:     100_000_000,
			wantSize:     455,
		},
		{
			name:         "high_cap_100mpps_on_800g",
			portSpeedBPS: 800_000_000_000,
			caps:         highCap,
			wantRate:     100_000_000,
			wantSize:     930,
		},
		{
			name:         "extreme_pps_clamped_to_64B_wire_rate_on_100g",
			portSpeedBPS: 100_000_000_000,
			caps:         extremePPS,
			wantRate:     141_369_047,
			wantSize:     64,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateMaxPPSParams(tc.portSpeedBPS, tc.caps, 0)
			if err != nil {
				t.Fatalf("CalculateMaxPPSParams() unexpected error: %v", err)
			}
			if got.PacketRate != tc.wantRate || got.PacketSize != tc.wantSize {
				t.Errorf("CalculateMaxPPSParams() got rate=%d size=%d, want rate=%d size=%d",
					got.PacketRate, got.PacketSize, tc.wantRate, tc.wantSize)
			}
			wireBPS := got.PacketRate * (uint64(got.PacketSize) + l1FramingOverheadBytes) * 8
			if wireBPS > tc.portSpeedBPS {
				t.Errorf("CalculateMaxPPSParams() exceeded port line rate: wireBPS=%d > portSpeed=%d", wireBPS, tc.portSpeedBPS)
			}
		})
	}
}

func TestCalculateDurations(t *testing.T) {
	genStandard := &plqpb.PacketGeneratorCapabilities{
		MinSetupDuration:    durationpb.New(30 * time.Second),
		MinTeardownDuration: durationpb.New(30 * time.Second),
		MinSampleInterval:   durationpb.New(10 * time.Second),
	}
	refCisco := &plqpb.ReflectorCapabilities{
		PmdLoopback: &plqpb.PmdLoopbackCapabilities{
			MinSetupDuration:    durationpb.New(210 * time.Second),
			MinTeardownDuration: durationpb.New(10 * time.Second),
		},
	}

	got, err := CalculateDurations(genStandard, refCisco, 0)
	if err != nil {
		t.Fatalf("CalculateDurations() unexpected error: %v", err)
	}
	if genReady, refReady := got.GenPreSync+got.GenSetup, got.RefPreSync+got.RefSetup; genReady <= refReady {
		t.Errorf("Generator becomes ready before reflector: genReady=%v <= refReady=%v", genReady, refReady)
	}
	if got.PollTimeout <= got.GenPreSync+got.GenSetup+got.Test+got.GenTeardown {
		t.Errorf("PollTimeout %v does not include grace buffer", got.PollTimeout)
	}
}

func TestValidateStatsPair(t *testing.T) {
	okStatus := &rpcpb.Status{Code: int32(codes.OK)}
	traffic := TrafficParams{PacketRate: 1_000_000, PacketSize: 512, Duration: 60 * time.Second}

	tests := []struct {
		name              string
		gen               *plqpb.QualificationResult
		ref               *plqpb.QualificationResult
		validateReflector bool
		wantErr           bool
	}{
		{
			name: "valid_with_reflector",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 60_000_000,
			},
			ref: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 60_000_000,
			},
			validateReflector: true,
			wantErr:           false,
		},
		{
			name: "zero_packets_rejected_no_nan_pass",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     0,
				PacketsReceived: 0,
			},
			validateReflector: false,
			wantErr:           true,
		},
		{
			name: "small_loss_above_0_0001_pct_rejected",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 59_999_000,
			},
			validateReflector: false,
			wantErr:           true,
		},
		{
			name: "packets_dropped_rejected",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 59_999_999,
				PacketsDropped:  1,
			},
			validateReflector: false,
			wantErr:           true,
		},
		{
			name: "low_delivered_rate_rejected",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     100_000,
				PacketsReceived: 100_000,
			},
			validateReflector: false,
			wantErr:           true,
		},
		{
			name: "reflector_mismatch_rejected_when_supported",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 60_000_000,
			},
			ref: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 30_000_000,
			},
			validateReflector: true,
			wantErr:           true,
		},
		{
			name: "reflector_zero_ignored_when_unsupported",
			gen: &plqpb.QualificationResult{
				Status:          okStatus,
				State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:     60_000_000,
				PacketsReceived: 60_000_000,
			},
			ref:               &plqpb.QualificationResult{},
			validateReflector: false,
			wantErr:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStatsPair(tc.gen, tc.ref, traffic, tc.validateReflector)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateStatsPair() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestComputeMinCapabilities(t *testing.T) {
	tests := []struct {
		name           string
		targetSpeedBPS uint64
		mtuDev         uint32
		ppsDev         uint64
		bpsDev         uint64
		want           CapabilityThresholds
	}{
		{
			name:           "default_100g",
			targetSpeedBPS: 100_000_000_000,
			want: CapabilityThresholds{
				MinMTU: 8184,
				MinPPS: 100_000_000,
				MinBPS: 100_000_000_000,
			},
		},
		{
			name:           "default_400g",
			targetSpeedBPS: 400_000_000_000,
			want: CapabilityThresholds{
				MinMTU: 8184,
				MinPPS: 100_000_000,
				MinBPS: 400_000_000_000,
			},
		},
		{
			name:           "default_800g",
			targetSpeedBPS: 800_000_000_000,
			want: CapabilityThresholds{
				MinMTU: 8184,
				MinPPS: 100_000_000,
				MinBPS: 800_000_000_000,
			},
		},
		{
			name:           "cisco_8202_explicit_bps_deviation",
			targetSpeedBPS: 400_000_000_000,
			mtuDev:         512,
			ppsDev:         13_000_000,
			bpsDev:         50_000_000_000,
			want: CapabilityThresholds{
				MinMTU: 512,
				MinPPS: 13_000_000,
				MinBPS: 50_000_000_000,
			},
		},
		{
			name:           "derived_bps_when_only_mtu_and_pps_deviations_set",
			targetSpeedBPS: 400_000_000_000,
			mtuDev:         512,
			ppsDev:         13_000_000,
			want: CapabilityThresholds{
				MinMTU: 512,
				MinPPS: 13_000_000,
				MinBPS: 512 * 13_000_000 * 8,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeMinCapabilities(tc.targetSpeedBPS, tc.mtuDev, tc.ppsDev, tc.bpsDev)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("computeMinCapabilities() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCalculateInjectorParams(t *testing.T) {
	tests := []struct {
		name    string
		caps    *plqpb.PacketInjectorCapabilities
		want    TrafficParams
		wantErr bool
	}{
		{
			name: "juniper_evo_asic_loopback",
			caps: &plqpb.PacketInjectorCapabilities{
				MinMtu:             64,
				MaxMtu:             9192,
				MinInjectedPackets: 1,
				MaxInjectedPackets: 120000,
				MinSampleInterval:  durationpb.New(10 * time.Second),
				LoopbackModes: []plqpb.PacketInjectorLoopbackMode{
					plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_ASIC,
				},
			},
			want: TrafficParams{
				PacketSize:       1500,
				Duration:         60 * time.Second,
				UseInjector:      true,
				PacketCount:      10,
				InjectorLoopback: plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_ASIC,
			},
		},
		{
			name: "pmd_only_fallback_and_clamped_count",
			caps: &plqpb.PacketInjectorCapabilities{
				MinMtu:             128,
				MaxMtu:             512,
				MinInjectedPackets: 25,
				MaxInjectedPackets: 50,
				LoopbackModes: []plqpb.PacketInjectorLoopbackMode{
					plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_PMD,
				},
			},
			want: TrafficParams{
				PacketSize:       512,
				Duration:         60 * time.Second,
				UseInjector:      true,
				PacketCount:      25,
				InjectorLoopback: plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_PMD,
			},
		},
		{
			name:    "nil_caps_fails",
			caps:    nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CalculateInjectorParams(tc.caps, 0)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CalculateInjectorParams() error = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("CalculateInjectorParams() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCalculateDurationsAsymmetricGenSetup(t *testing.T) {
	genSlow := &plqpb.PacketGeneratorCapabilities{
		MinSetupDuration:    durationpb.New(210 * time.Second),
		MinTeardownDuration: durationpb.New(30 * time.Second),
		MinSampleInterval:   durationpb.New(10 * time.Second),
	}
	refFast := &plqpb.ReflectorCapabilities{
		AsicLoopback: &plqpb.AsicLoopbackCapabilities{
			MinSetupDuration:    durationpb.New(30 * time.Second),
			MinTeardownDuration: durationpb.New(90 * time.Second),
		},
	}

	got, err := CalculateDurations(genSlow, refFast, 60*time.Second)
	if err != nil {
		t.Fatalf("CalculateDurations() unexpected error: %v", err)
	}
	genEnd := got.GenPreSync + got.GenSetup + got.Test + got.GenPostSync
	refActiveEnd := got.RefPreSync + got.RefSetup + got.Test + got.RefPostSync
	if refActiveEnd <= genEnd {
		t.Errorf("Reflector exits loopback before generator finishes: refActiveEnd=%v <= genEnd=%v", refActiveEnd, genEnd)
	}
}

func TestValidateStatsPairInjector(t *testing.T) {
	okStatus := &rpcpb.Status{Code: int32(codes.OK)}
	injParams := TrafficParams{
		PacketSize:  1500,
		Duration:    60 * time.Second,
		UseInjector: true,
		PacketCount: 10,
	}

	validGen := &plqpb.QualificationResult{
		Status:          okStatus,
		State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
		PacketsSent:     100_000_000,
		PacketsReceived: 100_000_000,
	}
	if err := ValidateStatsPair(validGen, validGen, injParams, true); err != nil {
		t.Errorf("ValidateStatsPair(injector valid) unexpected error: %v", err)
	}

	tooFew := &plqpb.QualificationResult{
		Status:          okStatus,
		State:           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
		PacketsSent:     5,
		PacketsReceived: 5,
	}
	if err := ValidateStatsPair(tooFew, tooFew, injParams, false); err == nil {
		t.Errorf("ValidateStatsPair(injector below packet_count) expected error, got nil")
	}

	zeroSent := &plqpb.QualificationResult{
		Status: okStatus,
		State:  plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
	}
	err := ValidateStatsPair(zeroSent, validGen, injParams, false)
	if err == nil || err.Error() != "injector reported 0 packets sent (expected >= 10)" {
		t.Errorf("ValidateStatsPair(injector zero sent) got err=%v, want \"injector reported 0 packets sent (expected >= 10)\"", err)
	}
}

func TestIsChannelizedJuniperInjector(t *testing.T) {
	withInj := &plqpb.CapabilitiesResponse{
		Generator: &plqpb.GeneratorCapabilities{
			PacketInjector: &plqpb.PacketInjectorCapabilities{MaxMtu: 9192},
		},
	}
	withoutInj := &plqpb.CapabilitiesResponse{
		Generator: &plqpb.GeneratorCapabilities{
			PacketGenerator: &plqpb.PacketGeneratorCapabilities{MaxMtu: 8184},
		},
	}

	tests := []struct {
		name   string
		vendor ondatra.Vendor
		port   string
		caps   *plqpb.CapabilitiesResponse
		want   bool
	}{
		{name: "juniper_channelized_with_caps", vendor: ondatra.JUNIPER, port: "et-0/0/1:3", caps: withInj, want: true},
		{name: "juniper_unchannelized", vendor: ondatra.JUNIPER, port: "et-0/0/1", caps: withInj, want: false},
		{name: "juniper_channelized_no_injector_caps", vendor: ondatra.JUNIPER, port: "et-0/0/1:3", caps: withoutInj, want: false},
		{name: "arista_ignored", vendor: ondatra.ARISTA, port: "Ethernet1/1", caps: withInj, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isChannelizedJuniperInjector(tc.vendor, tc.port, tc.caps); got != tc.want {
				t.Errorf("isChannelizedJuniperInjector() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewGeneratorQualificationConfig(t *testing.T) {
	d := Durations{
		GenPreSync:  20 * time.Second,
		GenSetup:    30 * time.Second,
		Test:        60 * time.Second,
		GenTeardown: 90 * time.Second,
	}
	injCfg := newGeneratorQualificationConfig("id-inj", "et-0/0/1:3", TrafficParams{
		PacketSize:       1500,
		UseInjector:      true,
		PacketCount:      10,
		InjectorLoopback: plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_ASIC,
	}, d)
	if injCfg.GetPacketInjector().GetPacketCount() != 10 || injCfg.GetPacketInjector().GetAsicLoopback() == nil {
		t.Errorf("Expected PacketInjector ASIC config, got %+v", injCfg)
	}

	genCfg := newGeneratorQualificationConfig("id-gen", "et-0/0/1", TrafficParams{
		PacketRate: 833333,
		PacketSize: 1500,
	}, d)
	if genCfg.GetPacketGenerator().GetPacketRate() != 833333 {
		t.Errorf("Expected PacketGenerator config, got %+v", genCfg)
	}
}

func TestSingleDUTAndTwoDUTAddressing(t *testing.T) {
	mustPrefix := func(ip string, length uint8) netip.Prefix {
		t.Helper()
		p, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", ip, length))
		if err != nil {
			t.Fatalf("ParsePrefix(%s/%d) failed: %v", ip, length, err)
		}
		return p.Masked()
	}

	var singleDUTPrefixes []netip.Prefix
	for idx := range 35 {
		a, b, err := linkAddresses(idx, true, fmt.Sprintf("p%d", idx*2+1), fmt.Sprintf("p%d", idx*2+2))
		if err != nil {
			t.Fatalf("linkAddresses(%d, true) unexpected error: %v", idx, err)
		}
		singleDUTPrefixes = append(singleDUTPrefixes,
			mustPrefix(a.ipv4, a.ipv4Len),
			mustPrefix(b.ipv4, b.ipv4Len),
			mustPrefix(a.ipv6, a.ipv6Len),
			mustPrefix(b.ipv6, b.ipv6Len),
		)
	}
	lagA, lagB, err := lagAddresses(true, "Port-Channel3", "Port-Channel4")
	if err != nil {
		t.Fatalf("lagAddresses(true) unexpected error: %v", err)
	}
	singleDUTPrefixes = append(singleDUTPrefixes,
		mustPrefix(lagA.ipv4, lagA.ipv4Len),
		mustPrefix(lagB.ipv4, lagB.ipv4Len),
		mustPrefix(lagA.ipv6, lagA.ipv6Len),
		mustPrefix(lagB.ipv6, lagB.ipv6Len),
	)

	seen := make(map[netip.Prefix]bool, len(singleDUTPrefixes))
	for _, p := range singleDUTPrefixes {
		if seen[p] {
			t.Fatalf("Duplicate single-DUT prefix %v in %v", p, singleDUTPrefixes)
		}
		seen[p] = true
	}

	var twoDUTPrefixes []netip.Prefix
	for idx := range 65 {
		a, b, err := linkAddresses(idx, false, "p1", "p1")
		if err != nil {
			t.Fatalf("linkAddresses(%d, false) unexpected error: %v", idx, err)
		}
		if mustPrefix(a.ipv4, a.ipv4Len) != mustPrefix(b.ipv4, b.ipv4Len) {
			t.Errorf("Expected two-DUT link %d endpoints to share IPv4 subnet: %v vs %v", idx, a.ipv4, b.ipv4)
		}
		if a.ipv4 == b.ipv4 {
			t.Errorf("Expected two-DUT link %d endpoints to have distinct IPs, both got %q", idx, a.ipv4)
		}
		twoDUTPrefixes = append(twoDUTPrefixes, mustPrefix(a.ipv4, a.ipv4Len))
	}
	twoLagA, _, err := lagAddresses(false, "agg1", "agg2")
	if err != nil {
		t.Fatalf("lagAddresses(false) unexpected error: %v", err)
	}
	twoDUTLagPrefix := mustPrefix(twoLagA.ipv4, twoLagA.ipv4Len)
	for _, p := range twoDUTPrefixes {
		if p == twoDUTLagPrefix {
			t.Fatalf("Two-DUT link prefix collided with LAG prefix %v", twoDUTLagPrefix)
		}
	}

	if _, _, err := linkAddresses(63, true, "p1", "p2"); err == nil {
		t.Errorf("linkAddresses(63, true) expected error beyond single-DUT slot limit, got nil")
	}
	if _, err := endpointAddressing(189, 1, "oob"); err == nil {
		t.Errorf("endpointAddressing(189) expected error for wraparound slot, got nil")
	}
}

func TestCalculateDurationsForVendorsCisco(t *testing.T) {
	genCaps := &plqpb.PacketGeneratorCapabilities{
		MinSetupDuration:    durationpb.New(time.Second),
		MinTeardownDuration: durationpb.New(time.Second),
		MinSampleInterval:   durationpb.New(10 * time.Second),
	}
	refCaps := &plqpb.ReflectorCapabilities{
		PmdLoopback: &plqpb.PmdLoopbackCapabilities{
			MinSetupDuration:    durationpb.New(time.Second),
			MinTeardownDuration: durationpb.New(time.Second),
		},
	}

	got, err := CalculateDurationsForVendors(ondatra.CISCO, ondatra.CISCO, genCaps, refCaps, 60*time.Second)
	if err != nil {
		t.Fatalf("CalculateDurationsForVendors() unexpected error: %v", err)
	}
	if got.RefPreSync != ciscoRefPreSync {
		t.Errorf("RefPreSync = %v, want %v", got.RefPreSync, ciscoRefPreSync)
	}
	if got.GenSetup < ciscoMinSetupDuration || got.RefSetup < ciscoMinSetupDuration {
		t.Errorf("Setup durations below Cisco floor %v: gen=%v, ref=%v", ciscoMinSetupDuration, got.GenSetup, got.RefSetup)
	}
	genEnd := got.GenPreSync + got.GenSetup + got.Test + got.GenPostSync
	refActiveEnd := got.RefPreSync + got.RefSetup + got.Test + got.RefPostSync
	if refActiveEnd <= genEnd {
		t.Errorf("Reflector exits loopback before generator finishes: refActiveEnd=%v <= genEnd=%v", refActiveEnd, genEnd)
	}
}

func TestIDsMatchListState(t *testing.T) {
	target := map[string]bool{"gen-1": true, "ref-1": true}
	onlyRef := &plqpb.ListResponse{
		Results: []*plqpb.ListResult{{Id: "ref-1"}},
	}
	both := &plqpb.ListResponse{
		Results: []*plqpb.ListResult{{Id: "gen-1"}, {Id: "ref-1"}, {Id: "other"}},
	}
	none := &plqpb.ListResponse{}

	if idsMatchListState(onlyRef, target, true) {
		t.Errorf("idsMatchListState(onlyRef, wantPresent=true) = true, want false")
	}
	if !idsMatchListState(both, target, true) {
		t.Errorf("idsMatchListState(both, wantPresent=true) = false, want true")
	}
	if idsMatchListState(onlyRef, target, false) {
		t.Errorf("idsMatchListState(onlyRef, wantPresent=false) = true, want false")
	}
	if !idsMatchListState(none, target, false) {
		t.Errorf("idsMatchListState(none, wantPresent=false) = false, want true")
	}
}

func TestResolveLAGMembers(t *testing.T) {
	dut1 := &ondatra.DUTDevice{}
	dut2 := &ondatra.DUTDevice{}
	p1 := &ondatra.Port{}
	p2 := &ondatra.Port{}

	t.Run("single_dut_one_pair", func(t *testing.T) {
		gotDUT1, gotDUT2, memA, memB, err := resolveLAGMembers([]LinkPair{
			{A: LinkEndpoint{DUT: dut1, Port: p1}, B: LinkEndpoint{DUT: dut1, Port: p2}},
		})
		if err != nil {
			t.Fatalf("resolveLAGMembers(single DUT 1-pair) unexpected error: %v", err)
		}
		if gotDUT1 != dut1 || gotDUT2 != dut1 {
			t.Errorf("got DUTs (%p, %p), want (%p, %p)", gotDUT1, gotDUT2, dut1, dut1)
		}
		if len(memA) != 1 || memA[0] != p1 || len(memB) != 1 || memB[0] != p2 {
			t.Errorf("got members (%v, %v), want ([%v], [%v])", memA, memB, p1, p2)
		}
	})

	t.Run("two_duts_one_pair", func(t *testing.T) {
		gotDUT1, gotDUT2, memA, memB, err := resolveLAGMembers([]LinkPair{
			{A: LinkEndpoint{DUT: dut1, Port: p1}, B: LinkEndpoint{DUT: dut2, Port: p2}},
		})
		if err != nil {
			t.Fatalf("resolveLAGMembers(two DUT 1-pair) unexpected error: %v", err)
		}
		if gotDUT1 != dut1 || gotDUT2 != dut2 {
			t.Errorf("got DUTs (%p, %p), want (%p, %p)", gotDUT1, gotDUT2, dut1, dut2)
		}
		if len(memA) != 1 || memA[0] != p1 || len(memB) != 1 || memB[0] != p2 {
			t.Errorf("got members (%v, %v), want ([%v], [%v])", memA, memB, p1, p2)
		}
	})

	t.Run("rejects_empty_pairs", func(t *testing.T) {
		if _, _, _, _, err := resolveLAGMembers(nil); err == nil {
			t.Errorf("resolveLAGMembers(nil) expected error, got nil")
		}
	})

	t.Run("rejects_mixed_duts", func(t *testing.T) {
		_, _, _, _, err := resolveLAGMembers([]LinkPair{
			{A: LinkEndpoint{DUT: dut1, Port: p1}, B: LinkEndpoint{DUT: dut2, Port: p2}},
			{A: LinkEndpoint{DUT: dut2, Port: p1}, B: LinkEndpoint{DUT: dut1, Port: p2}},
		})
		if err == nil {
			t.Errorf("resolveLAGMembers(mixed DUTs) expected error, got nil")
		}
	})

	t.Run("rejects_self_looped_port", func(t *testing.T) {
		if _, _, _, _, err := resolveLAGMembers([]LinkPair{
			{A: LinkEndpoint{DUT: dut1, Port: p1}, B: LinkEndpoint{DUT: dut1, Port: p1}},
		}); err == nil {
			t.Errorf("resolveLAGMembers(self-looped port) expected error, got nil")
		}
	})
}

func TestValidateDeleteResults(t *testing.T) {
	okResp := &plqpb.DeleteResponse{
		Results: map[string]*rpcpb.Status{
			"id-1": {Code: int32(codes.OK)},
		},
	}
	canceledResp := &plqpb.DeleteResponse{
		Results: map[string]*rpcpb.Status{
			"id-1": {Code: int32(codes.Canceled)},
		},
	}
	errResp := &plqpb.DeleteResponse{
		Results: map[string]*rpcpb.Status{
			"id-1": {Code: int32(codes.NotFound)},
		},
	}

	if canceled, errs := validateDeleteResults("dut1", []string{"id-1"}, okResp, false); len(canceled) != 0 || len(errs) != 0 {
		t.Errorf("okResp got canceled=%v errs=%v, want empty", canceled, errs)
	}
	if canceled, errs := validateDeleteResults("dut1", []string{"id-1"}, canceledResp, true); len(canceled) != 1 || canceled[0] != "id-1" || len(errs) != 0 {
		t.Errorf("canceledResp(allowCanceled=true) got canceled=%v errs=%v, want ([id-1], nil)", canceled, errs)
	}
	if _, errs := validateDeleteResults("dut1", []string{"id-1"}, canceledResp, false); len(errs) != 1 {
		t.Errorf("canceledResp(allowCanceled=false) got %d errs, want 1", len(errs))
	}
	if _, errs := validateDeleteResults("dut1", []string{"id-1"}, errResp, true); len(errs) != 1 {
		t.Errorf("errResp got %d errs, want 1", len(errs))
	}
	if _, errs := validateDeleteResults("dut1", []string{"missing"}, okResp, true); len(errs) != 1 {
		t.Errorf("missing id got %d errs, want 1", len(errs))
	}
}
