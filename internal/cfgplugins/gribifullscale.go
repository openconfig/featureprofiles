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

// Package cfgplugins provides shared helpers for gRIBI full-scale tests
// TE-14.3 (T1) and TE-14.4 (T2). All topology-configuration, gRIBI-programming,
// verification, and traffic functions live here so that the two test packages
// only need to supply their scale-specific constants.
package cfgplugins

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	spb "github.com/openconfig/gnoi/system"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// ============================================================
// Constants — shared across T1 and T2
// ============================================================

const (
	// MTU for DUT/ATE interfaces.
	MTU = 9216

	// Port1 DUT/ATE addresses — single /30 sub-interface.
	DUTPort1IPv4 = "192.0.2.1"
	ATEPort1IPv4 = "192.0.2.2"
	DUTPort1IPv6 = "2001:0:0:1::1"
	ATEPort1IPv6 = "2001:0:0:2::2"
	ATEPort1MAC  = "02:00:01:01:01:01"

	// Port2: 640 VLAN-tagged /30 sub-interfaces carved from 198.18.0.0/20.
	DUTPort2IPv4    = "193.0.2.1"
	ATEPort2IPv4    = "193.0.2.2"
	DUTPort2IPv6    = "3001:0:0:1::1"
	ATEPort2IPv6    = "3001:0:0:2::2"
	ATEPort2MAC     = "02:00:02:00:00:01"
	ATEPort2MACStep = "00:00:00:00:01:00"

	DUTPort1IPv4Start = "192.51.0.1"
	ATEPort1IPv4Start = "192.51.0.2"
	DUTPort2IPv4Start = "193.51.0.1"
	ATEPort2IPv4Start = "193.51.0.2"
	DUTPort1IPv6Start = "4001:0:0:1::1"
	ATEPort1IPv6Start = "4001:0:0:1::2"
	DUTPort2IPv6Start = "5001:0:0:1::1"
	ATEPort2IPv6Start = "5001:0:0:1::2"
	PortIPv6Step      = "0:0:0:1::"
	PortIPv4Step      = "0.0.0.4"
	StartVLANPort1    = 100
	StartVLANPort2    = 200
	FIBPrgCount       = 5
	// ShutdownVLANPct is the percentage of subinterfaces to shut down during repair tests.
	ShutdownVLANPct = 50
	// Prefix mask constants.
	IPv4IntfMask = 30
	IPv6IntfMask = 126
	IPv4HostMask = 32
	IPv6HostMask = 128

	// VRF / policy names.
	VRFPolC          = "vrf_selection_policy_c"
	DecapVRFStr      = "DECAP_TE_VRF"
	TransitVRF111Str = "TE_VRF_111"
	TransitVRF222Str = "TE_VRF_222"
	RepairVRFStr     = "REPAIR_VRF"

	// Magic source IPs per README.
	IPv4OuterSrc111 = "198.51.100.111"
	IPv4OuterSrc222 = "198.51.100.222"

	// Inner destination IPs for traffic validation.
	DecapIPv4InnerDst   = "172.16.0.1"
	EncapIPv6InnerSrc   = "2001:db8:ffff::1"
	TransitIPv4InnerDst = "172.16.0.2"
	RepairIPv4InnerDst  = "172.16.0.3"

	// Default VRF scale — identical for T1 and T2.
	IPv4PrefixStartAddress = "10.0.0.1"

	// Prefix start addresses for Transit and Repair VRFs.
	TransitVRF111PrefixStart = "100.0.0.1"
	TransitVRF222PrefixStart = "101.0.0.1"

	// Common prefix step used across multiple VRF builders.
	CommonPrefixStep     = "0.0.0.1"
	CommonIPv6PrefixStep = "::1"

	// gRIBI batch programming parameters.
	DefaultGRIBIBatchSize = 2_000

	// NH/NHG ID base constants — kept non-overlapping across all VRFs.
	NHBaseDefault  = uint64(1_000)
	NHGBaseDefault = uint64(2_000)
	NHBaseTransit  = uint64(10_000)
	NHGBaseTransit = uint64(20_000)
	NHBaseRepair   = uint64(30_000)
	NHGBaseRepair  = uint64(32_000)
	NHBaseEncap    = uint64(50_000)
	NHGBaseEncap   = uint64(82_000)
	NHBaseDecap    = uint64(120_000)
	NHGBaseDecap   = uint64(120_100)

	// Static NHG IDs for S1/S2.
	StaticS1NHG   = uint64(9001)
	StaticS2NHG   = uint64(9002)
	RandomIPCheck = 100

	// Chassis reboot constants.
	OneSecondInNanoSecond = 1e9
	RebootDelay           = 30
	MaxRebootTime         = 900
	MaxCompWaitTime       = 900

	// Fallback VLAN ID for subinterface 0 if NoMixOfTaggedAndUntaggedSubinterfaces is true.
	NoMixVlanIDBase = 10
)

// ============================================================
// Package-level variables
// ============================================================

var (
	excludeTraffic = flag.Bool("exclude_traffic", false, "Exclude traffic generation and ATE/IXIA device interaction, testing only DUT gRIBI programming.")

	// dutPort1Attr and atePort1Attr hold port1 L3 attributes.
	dutPort1Attr = attrs.Attributes{
		Name:    "DUT Ingress Port1",
		Desc:    "dutPort1",
		IPv4:    DUTPort1IPv4,
		IPv4Len: IPv4IntfMask,
		IPv6:    DUTPort1IPv6,
		IPv6Len: IPv6IntfMask,
		MTU:     MTU,
	}

	atePort1Attr = attrs.Attributes{
		Name:    "ATE-Ingress-Port1",
		MAC:     ATEPort1MAC,
		IPv4:    ATEPort1IPv4,
		IPv4Len: IPv4IntfMask,
		IPv6:    ATEPort1IPv6,
		IPv6Len: IPv6IntfMask,
	}

	dutPort2Attr = attrs.Attributes{
		Name:    "DUT Ingress Port2",
		Desc:    "dutPort2",
		IPv4:    DUTPort2IPv4,
		IPv4Len: IPv4IntfMask,
		IPv6:    DUTPort2IPv6,
		IPv6Len: IPv6IntfMask,
		MTU:     MTU,
	}

	atePort2Attr = attrs.Attributes{
		Name:    "ATE-Ingress-Port2",
		MAC:     ATEPort2MAC,
		IPv4:    ATEPort2IPv4,
		IPv4Len: IPv4IntfMask,
		IPv6:    ATEPort2IPv6,
		IPv6Len: IPv6IntfMask,
	}

	// decapPrefixLens is the mix of prefix lengths for DECAP_TE_VRF.
	decapPrefixLens = []int{22, 24, 26, 28}

	// outerSrcs enumerates the two magic outer source IPs across traffic scenarios.
	outerSrcs = []string{IPv4OuterSrc111, IPv4OuterSrc222}
)

// ============================================================
// Types
// ============================================================

// TrafficScenario describes the expected packet forwarding behaviour.
type TrafficScenario int

// TrafficScenario enumeration values.
const (
	ScenarioEncap    TrafficScenario = iota // plain in → encapsulated out
	ScenarioDecap                           // encapsulated in → plain inner out
	ScenarioReencap                         // decap outer + re-encap with new outer
	ScenarioTransit                         // encapsulated passthrough via TE_VRF_111
	ScenarioRepaired                        // encapsulated passthrough via TE_VRF_222
)

// FlowExpectation carries the per-flow validation criteria used in
// ValidateCapturedPackets. All fields are populated by RunEndToEndTrafficValidation
// before traffic starts so that the validation logic is fully declarative.
type FlowExpectation struct {
	Scenario TrafficScenario
	// ExpectedOuterSrc is the IPv4 source address that the DUT stamps as the
	// outer-src on encap/transit/repaired egress packets. Passed as IPv4Layer.DstIP
	// to packetvalidationhelpers.CaptureAndValidatePackets (which checks DstIP
	// inside validateIPv4Header). Empty for Decap (no outer tunnel header).
	ExpectedOuterSrc string
	// ExpectedDSCPs is the set of DSCP values the DUT must preserve or stamp on
	// egress. ValidateCapturedPackets spot-checks the first value in this set
	// via the outer header's TOS byte (TOS = DSCP << 2). Both DSCP values for
	// the VRF are valid because OTG cycles SetValues across packets.
	ExpectedDSCPs []uint32
	// WantEncapPresent indicates that the egress packet must carry an outer
	// IP-in-IP header (Scenario determines this implicitly; retained for clarity).
	WantEncapPresent bool
	// DecapPrefixSet is the set of outer-dst prefixes used in decap/reencap
	// flows; retained for documentation — reencap dst validation uses the
	// Scenario field rather than a prefix set comparison in the helper call.
	DecapPrefixSet []string
}

// NHGLoadBalancingParams specifies the percentage of NextHopGroups in a VRF
// that should load-balance across a given number of NextHops.
type NHGLoadBalancingParams struct {
	// Pct is the percentage of NextHopGroups in a VRF that should load-balance
	// across a given number of NextHops.
	Pct int
	// NumNextHops is the number of NextHops that a NextHopGroup should
	// load-balance across.
	NumNextHops int
}

type nhgLoadBalancingBucket struct {
	numNHG             int
	numLoadBalancingNH int
}

// WeightConfig is a sealed interface representing next-hop group weight scaling configurations.
type WeightConfig interface {
	isWeightConfig()
}

// ECMP represents standard ECMP load balancing where all next-hops have equal weights (1).
type ECMP struct{}

func (ECMP) isWeightConfig() {}

// WCMP represents weighted cost multi-path load balancing with a custom weight granularity.
type WCMP struct {
	WeightGranularity uint64
}

func (WCMP) isWeightConfig() {}

// ExplicitWeights defines an explicit list of weights for next-hops in a NextHopGroup.
type ExplicitWeights struct {
	Weights []uint64
}

func (ExplicitWeights) isWeightConfig() {}

var (
	// WCMP1 is a WCMP configuration representing a granularity of 1.
	WCMP1 = WCMP{WeightGranularity: 1}
	// WCMP1in32 is a WCMP configuration representing a granularity of 1/32.
	WCMP1in32 = WCMP{WeightGranularity: 32}
	// WCMP1in64 is a WCMP configuration representing a granularity of 1/64.
	WCMP1in64 = WCMP{WeightGranularity: 64}
	// WCMP1in128 is a WCMP configuration representing a granularity of 1/128.
	WCMP1in128 = WCMP{WeightGranularity: 128}
	// WCMP1in256 is a WCMP configuration representing a granularity of 1/256.
	WCMP1in256 = WCMP{WeightGranularity: 256}
	// WCMP1in512 is a WCMP configuration representing a granularity of 1/512.
	WCMP1in512 = WCMP{WeightGranularity: 512}
	// WCMP1in1024 is a WCMP configuration representing a granularity of 1/1024.
	WCMP1in1024 = WCMP{WeightGranularity: 1024}
)

// NHGWeightParams specifies the percentage of NextHopGroups in a VRF
// that should have a given weight configuration (ECMP or WCMP with granularity).
type NHGWeightParams struct {
	// Pct is the percentage of NextHopGroups.
	Pct int
	// Config defines either ECMP{} or WCMP{WeightGranularity: ...}.
	Config WeightConfig
}

// NHGBucketIterator generates the sequence of next-hop count and weight specification weights
// for NextHopGroups, according to specified load-balancing and weight configurations.
type NHGBucketIterator struct {
	lbBuckets       []nhgLoadBalancingBucket
	weightBuckets   []nhgWeightBucket
	maxNHsAvailable int

	lbIdx         int
	lbCreated     int
	weightIdx     int
	weightCreated int
}

// NewNHGBucketIterator creates a new initialized NHGBucketIterator.
func NewNHGBucketIterator(numNHG int, lbParams []NHGLoadBalancingParams, weightParams []NHGWeightParams, maxNHsAvailable int) *NHGBucketIterator {
	return &NHGBucketIterator{
		lbBuckets:       computeNHGBuckets(numNHG, lbParams),
		weightBuckets:   computeNHGWeightBuckets(numNHG, weightParams),
		maxNHsAvailable: maxNHsAvailable,
	}
}

// Next advances the iterator and returns the slice of next-hop weights for the next group.
func (it *NHGBucketIterator) Next() []uint64 {
	if it.lbIdx < len(it.lbBuckets)-1 && it.lbCreated >= it.lbBuckets[it.lbIdx].numNHG {
		it.lbIdx++
		it.lbCreated = 0
	}
	targetNHCount := it.lbBuckets[it.lbIdx].numLoadBalancingNH
	actualNHCount := min(targetNHCount, it.maxNHsAvailable)

	if it.weightIdx < len(it.weightBuckets)-1 && it.weightCreated >= it.weightBuckets[it.weightIdx].numNHG {
		it.weightIdx++
		it.weightCreated = 0
	}
	cfg := it.weightBuckets[it.weightIdx].config

	it.lbCreated++
	it.weightCreated++

	switch c := cfg.(type) {
	case ECMP:
		weights := make([]uint64, actualNHCount)
		for idx := 0; idx < actualNHCount; idx++ {
			weights[idx] = 1
		}
		return weights
	case WCMP:
		return computeNHGWeights(c.WeightGranularity, actualNHCount)
	case ExplicitWeights:
		weights := make([]uint64, actualNHCount)
		for idx := 0; idx < actualNHCount; idx++ {
			if idx < len(c.Weights) {
				weights[idx] = c.Weights[idx]
			} else {
				weights[idx] = 1
			}
		}
		return weights
	default:
		weights := make([]uint64, actualNHCount)
		for idx := 0; idx < actualNHCount; idx++ {
			weights[idx] = 1
		}
		return weights
	}
}

type nhgWeightBucket struct {
	numNHG int
	config WeightConfig
}

// validateNHGLoadBalance verifies that the next hop group load-balancing specifications are valid.
// Specifically, it checks that percentages are non-negative, next-hop counts are positive, and the total percentage sum equals exactly 100.
func validateNHGLoadBalance(t *testing.T, name string, configs []NHGLoadBalancingParams) {
	t.Helper()
	if len(configs) == 0 {
		t.Fatalf("validateNHGLoadBalance: %s is empty", name)
	}
	sumPct := 0
	for _, spec := range configs {
		if spec.Pct < 0 {
			t.Fatalf("validateNHGLoadBalance: invalid negative percentage (%d) in %s", spec.Pct, name)
		}
		if spec.NumNextHops <= 0 {
			t.Fatalf("validateNHGLoadBalance: invalid next-hops count (%d) in %s", spec.NumNextHops, name)
		}
		sumPct += spec.Pct
	}
	if sumPct != 100 {
		t.Fatalf("validateNHGLoadBalance: sum of percentages in %s must be exactly 100, got %d", name, sumPct)
	}
}

// partitionNHG distributes the next hop group count into buckets based on the percentage requirements,
// allocating any remainder to the final bucket.
func partitionNHG[T any](numNHG int, req []T, getPct func(T) int) []int {
	counts := make([]int, len(req))
	totalNHG := 0
	for idx, spec := range req {
		numNH := numNHG * getPct(spec) / 100
		counts[idx] = numNH
		totalNHG += numNH
	}
	counts[len(counts)-1] += numNHG - totalNHG
	return counts
}

// computeNHGBuckets converts the NHG load balancing spec from percentage-based to
// absolute values. Each bucket contains a set of NHGs and the number of load
// balancing NHs that each NHG should use.
func computeNHGBuckets(numNHG int, req []NHGLoadBalancingParams) []nhgLoadBalancingBucket {
	counts := partitionNHG(numNHG, req, func(s NHGLoadBalancingParams) int { return s.Pct })
	buckets := make([]nhgLoadBalancingBucket, len(req))
	for idx, spec := range req {
		buckets[idx] = nhgLoadBalancingBucket{
			numNHG:             counts[idx],
			numLoadBalancingNH: spec.NumNextHops,
		}
	}
	return buckets
}

// AreAllNHsReferenced returns true if every next hop from 0 to numNH-1 is referenced
// by at least one next hop group under the given load balancing parameters, and false otherwise.
func AreAllNHsReferenced(numNH int, numNHG int, lbParams []NHGLoadBalancingParams) bool {
	if numNH <= 0 || numNHG <= 0 || len(lbParams) == 0 {
		return false
	}
	buckets := computeNHGBuckets(numNHG, lbParams)
	totalSlots := 0
	for _, b := range buckets {
		actualNHCount := min(b.numLoadBalancingNH, numNH)
		totalSlots += b.numNHG * actualNHCount
	}
	return totalSlots >= numNH
}

// computeNHGWeightBuckets converts the NHG target weight sum spec from percentage-based to
// absolute group counts.
func computeNHGWeightBuckets(numNHG int, req []NHGWeightParams) []nhgWeightBucket {
	counts := partitionNHG(numNHG, req, func(s NHGWeightParams) int { return s.Pct })
	buckets := make([]nhgWeightBucket, len(req))
	for idx, spec := range req {
		buckets[idx] = nhgWeightBucket{
			numNHG: counts[idx],
			config: spec.Config,
		}
	}
	return buckets
}

// validateNHGWeight verifies that next hop group weight specifications are valid.
func validateNHGWeight(t *testing.T, name string, configs []NHGWeightParams) {
	t.Helper()
	if len(configs) == 0 {
		t.Fatalf("validateNHGWeight: %s is empty", name)
	}
	sumPct := 0
	for _, spec := range configs {
		if spec.Pct < 0 {
			t.Fatalf("validateNHGWeight: invalid negative percentage (%d) in %s", spec.Pct, name)
		}
		if spec.Config == nil {
			t.Fatalf("validateNHGWeight: WeightConfig cannot be nil in %s", name)
		}
		switch cfg := spec.Config.(type) {
		case ECMP:
			// valid
		case WCMP:
			if cfg.WeightGranularity == 0 {
				t.Fatalf("validateNHGWeight: WeightGranularity must be greater than 0 for WCMP in %s", name)
			}
		case ExplicitWeights:
			if len(cfg.Weights) == 0 {
				t.Fatalf("validateNHGWeight: ExplicitWeights cannot be empty in %s", name)
			}
		default:
			t.Fatalf("validateNHGWeight: unexpected type %T for WeightConfig in %s", spec.Config, name)
		}
		sumPct += spec.Pct
	}
	if sumPct != 100 {
		t.Fatalf("validateNHGWeight: sum of percentages in %s must be exactly 100, got %d", name, sumPct)
	}
}

// computeNHGWeights calculates next-hop weights for actualNHCount next-hops given a WeightGranularity.
// To prevent standard ECMP from being programmed by the router and force WCMP, the next-hop
// weights must be slightly uneven. This function distributes the granularity as evenly as possible
// and shifts 1 weight unit from the second-to-last next hop to the last next hop if all weights are equal.
func computeNHGWeights(granularity uint64, actualNHCount int) []uint64 {
	weights := make([]uint64, actualNHCount)
	if actualNHCount == 0 {
		return weights
	}
	baseWeight := granularity / uint64(actualNHCount)
	remainder := granularity % uint64(actualNHCount)
	for i := 0; i < actualNHCount; i++ {
		w := baseWeight
		if i == actualNHCount-1 {
			w += remainder
		}
		weights[i] = w
	}
	// Force uneven weights to trigger WCMP instead of standard ECMP if baseWeight is greater than 1.
	if actualNHCount > 1 && baseWeight > 1 {
		allEqual := true
		for idx := 1; idx < actualNHCount; idx++ {
			if weights[idx] != weights[0] {
				allEqual = false
				break
			}
		}
		if allEqual {
			weights[actualNHCount-2]--
			weights[actualNHCount-1]++
		}
	}
	return weights
}

// ScaleParams holds the scale constants that configure the gRIBI full scale setup.
type ScaleParams struct {
	GRIBIBatchSize    int
	GRIBIBatchTimeout time.Duration

	// The number of NextHops in Default VRF egressing the switch.
	// Each NH will point to a unique output VLAN subinterface.
	// Based on ShutdownVLANPct, these NHs are virtually split in 2 sub-groups:
	// 1) primary NHs pointing to VLANs that will NOT be shut down.
	// 2) backup NHs pointing to VLANs that will be shut down during the repair tests.
	NumDefaultNH int
	// The number of NextHopGroups in Default VRF egressing the switch.
	// Each NHG reference and load-balance across the NH created via `NumDefaultNH`.
	// Also, virtually split into 2 sub-groups:
	// 1) primary NHGs containing primary NHs.
	// 2) backup NHGs containing backup NHs.
	NumDefaultNHG int
	// The load-balancing parameters for NextHopGroups in Default VRF.
	// e.g. DefaultNHGLoadBalance: []cfgplugins.NHGLoadBalancingParams{
	// 	{Pct: 40, NumNextHops: 8},
	// 	{Pct: 40, NumNextHops: 16},
	// 	{Pct: 15, NumNextHops: 32},
	// 	{Pct: 5, NumNextHops: 64},
	// },
	DefaultNHGLoadBalance []NHGLoadBalancingParams

	// The target weight sum distribution parameters for NextHopGroups in Default VRF.
	// e.g. DefaultNHGWeight: []cfgplugins.NHGWeightParams{
	// 	{Pct: 80, TargetWeightSum: 512},
	// 	{Pct: 20, TargetWeightSum: 1024},
	// },
	DefaultNHGWeight []NHGWeightParams
	// The number of fictitious IPv4 prefixes in Default VRF.
	// Theese IP entries will be pointed by the transit routes.
	// Each prefix points to a unique NHG. Also virtually split into 2 sub-groups:
	// 1) primary prefixes pointing to primary NHGs.
	// 2) backup prefixes pointing to backup NHGs.
	NumDefaultIPv4 int

	// The number of NextHops in transit VRFs (transit TE_VRF_111 and repaired TE_VRF_222) pointing
	// to fictitious IPv4 prefixes in Default VRF.
	// The NHs will be split in 2 sub-groups
	// 1) NHs used in transit VRF pointing to primary NHs pointing to primary default prefixes
	// 2) NHs used in repaired VRF pointing to backup NHs pointing to backup default prefixes.
	NumTransitNH int
	// The number of NextHopgroups in transit VRFs (transit TE_VRF_111 and repaired TE_VRF_222)
	// Each NHG reference and load-balance across 2 NHs created via `NumTransitNH` with 1:63 weight.
	// The NHGs will be split in 2 sub-groups
	// 1) NHGs used in transit VRF referencing NHs in sub-group 1)
	// 2) NHGs used in repaired VRF referencing NHs in sub-group 2)
	NumTransitNHG int
	// The load-balancing parameters for NextHopGroups in transit VRFs.
	// The most common case is 100% of NHGs load-balancing across 2 NHs (1:63 weight) to simulate
	// the load-balancing across self-site and next-site.
	// e.g. TransitNHGLoadBalance: []cfgplugins.NHGLoadBalancingParams{
	// 	{Pct: 100, NumNextHops: 2},
	// },
	TransitNHGLoadBalance []NHGLoadBalancingParams
	// The target weight sum distribution parameters for NextHopGroups in transit VRFs.
	// e.g. TransitNHGWeight: []cfgplugins.NHGWeightParams{
	// 	{Pct: 100, TargetWeightSum: 64},
	// },
	TransitNHGWeight []NHGWeightParams
	// The number of IPv4 prefixes in each of the 2 transit VRFs (transit TE_VRF_111 and repaired TE_VRF_222)
	// The IPv4 entries in transit VRF will point to NHGs in sub-group 1)
	// The IPv4 entries in repaired VRF will point to NHGs in sub-group 2)
	NumTransitIPv4 int

	// The number of IPv4 prefixes in the repair VRF
	NumRepairIPv4 int
	// The number of NextHopGroups in the repair VRF
	NumRepairNHG int

	NumEncapVRFs      int
	NumEncapNHPerVRF  int
	NumEncapNHGPerVRF int
	// The load-balancing parameters for NextHopGroups in Encap VRF.
	// e.g. EncapNHGLoadBalance: []cfgplugins.NHGLoadBalancingParams{
	// 	{Pct: 75, NumNextHops: 4},
	// 	{Pct: 20, NumNextHops: 8},
	// 	{Pct: 3, NumNextHops: 16},
	// 	{Pct: 2, NumNextHops: 32},
	// },
	EncapNHGLoadBalance []NHGLoadBalancingParams
	// The target weight sum distribution parameters for NextHopGroups in Encap VRF.
	// e.g. EncapNHGWeight: []cfgplugins.NHGWeightParams{
	// 	{Pct: 75, TargetWeightSum: 32},
	// 	{Pct: 20, TargetWeightSum: 64},
	// 	{Pct: 3, TargetWeightSum: 128},
	// 	{Pct: 2, TargetWeightSum: 256},
	// },
	EncapNHGWeight     []NHGWeightParams
	NumEncapIPv4PerVRF int
	NumEncapIPv6PerVRF int

	NumDecapEntries     int
	DecapDestsSubsetPct int

	TrafficDuration time.Duration
	TrafficLossTol  uint64
	TrafficRateMpps uint64

	NumPort2VLANs int
}

// TrafficTestCase is a table-driven entry for the two traffic profiles.
type TrafficTestCase struct {
	Name       string
	UseIMIX    bool
	TestRepair bool
}

// String returns a human-readable name for the scenario.
func (s TrafficScenario) String() string {
	switch s {
	case ScenarioEncap:
		return "Encap"
	case ScenarioDecap:
		return "Decap"
	case ScenarioReencap:
		return "Reencap"
	case ScenarioTransit:
		return "Transit"
	case ScenarioRepaired:
		return "Repaired"
	default:
		return fmt.Sprintf("scenario(%d)", int(s))
	}
}

// ============================================================
// VRF name builders
// ============================================================

// BuildEncapVRFNames returns the encap VRF names dynamically based on count.
func BuildEncapVRFNames(numEncapVRFs int) []string {
	v := make([]string, numEncapVRFs)
	for i := range v {
		v[i] = fmt.Sprintf("ENCAP_TE_VRF_%c", 'A'+i)
	}
	return v
}

// BuildAllNonDefaultVRFs returns the complete list of non-default VRFs.
func BuildAllNonDefaultVRFs(encapVRFs []string) []string {
	v := append([]string{}, encapVRFs...)
	v = append(v, TransitVRF111Str, TransitVRF222Str, RepairVRFStr, DecapVRFStr)
	return v
}

// EncapVRFDSCP returns the pair of DSCP values for encapVRFs[i].
func EncapVRFDSCP(vrfIdx int) (uint8, uint8) {
	return uint8(10 + vrfIdx*2), uint8(11 + vrfIdx*2)
}

// EncapDSCPVal returns a DSCP as uint32 for flow headers.
func EncapDSCPVal(vrfIdx, variant int) uint32 {
	d1, d2 := EncapVRFDSCP(vrfIdx)
	if variant == 0 {
		return uint32(d1)
	}
	return uint32(d2)
}

// getAllEncapDSCPVals returns all DSCP values across all encap VRFs.
func getAllEncapDSCPVals(numEncapVRFs int) []uint32 {
	dscpVals := make([]uint32, 0, 2*numEncapVRFs)
	for vi := 0; vi < numEncapVRFs; vi++ {
		d1, d2 := EncapVRFDSCP(vi)
		dscpVals = append(dscpVals, uint32(d1), uint32(d2))
	}
	return dscpVals
}

// ConfigureDUT sets up port interfaces, VRFs, and VRF-selection policy,
// and returns the instantiated DUT device along with its default network instance name.
func ConfigureDUT(t *testing.T, params ScaleParams) (*ondatra.DUTDevice, string) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	defaultVRF := deviations.DefaultNetworkInstance(dut)
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := gnmi.OC()
	vrfBatch := new(gnmi.SetBatch)

	ConfigureHardwareInit(t, dut)

	CreateGRIBIScaleVRFs(t, dut, vrfBatch, params.NumEncapVRFs)

	inputPortList := []*ondatra.Port{dp1}
	dutInputPortAttrs := []attrs.Attributes{dutPort1Attr}

	outputPortList := []*ondatra.Port{dp2}
	dutOutputPortAttrs := []attrs.Attributes{dutPort2Attr}

	for idx, a := range dutInputPortAttrs {
		configureDUTInterface(t, vrfBatch, dut, inputPortList[idx], a, false)
	}
	for idx, a := range dutOutputPortAttrs {
		configureDUTInterface(t, vrfBatch, dut, outputPortList[idx], a, true)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		t.Log("Configure/update Network Instance type")
		dutConfNIPath := d.NetworkInstance(defaultVRF)
		gnmi.BatchUpdate(vrfBatch, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	}
	// Configure sub-interfaces on port2
	ConfigureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp2, DUTPort2IPv4Start, DUTPort2IPv6Start, StartVLANPort2, params.NumPort2VLANs)
	vrfBatch.Set(t, dut)

	ConfigureCLIDecapVRFMode(t, dut)
	encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
	ConfigureVRFSelectionPolicyOC(t, dut, encapVRFNames)

	return dut, defaultVRF
}

// configureDUTInterface sets up a physical port interface, including MTU, optional base subinterface VLAN matching, and default network instance assignment.
func configureDUTInterface(t *testing.T, vrfBatch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, a attrs.Attributes, configureVlan bool) {
	t.Helper()
	d := gnmi.OC()
	intf := a.NewOCInterface(p.Name(), dut)
	if !deviations.OmitL2MTU(dut) && a.MTU > 0 {
		// Physical MTU (L2) = L3 MTU + Ethernet header size (14) + VLAN ID (4) + FrameCheckSequence (4) = 22 bytes
		l2MTUDiff := uint16(22)
		intf.Mtu = ygot.Uint16(uint16(a.MTU) + l2MTUDiff)
	}
	if configureVlan && deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		s := intf.GetOrCreateSubinterface(a.Subinterface)
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(NoMixVlanIDBase)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(NoMixVlanIDBase)
		}
	}
	gnmi.BatchUpdate(vrfBatch, d.Interface(p.Name()).Config(), intf)
	assignInterfaceToDefaultNI(t, vrfBatch, dut, p.Name(), uint32(a.Subinterface))
	t.Logf("Configured DUT port %s (%s)", p.Name(), a.Desc)
}

// ConfigureDUTSubinterfaces creates multiple VLAN-tagged sub-interfaces on dut port, deriving IPv4/IPv6 addresses from the provided prefixes.
func ConfigureDUTSubinterfaces(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root,
	dut *ondatra.DUTDevice, dutPort *ondatra.Port,
	prefixFmtV4, prefixFmtV6 string, startVLANPort, subIntCount int) {
	t.Helper()
	dutIPsV4, err := iputil.GenerateIPsWithStep(prefixFmtV4, subIntCount, PortIPv4Step)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv4s: %v", err)
	}
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(prefixFmtV6, subIntCount, PortIPv6Step)
	if err != nil {
		t.Fatalf("failed to generate DUT IPv6s: %v", err)
	}
	for i := range subIntCount {
		index := uint32(i + 1)
		vlanID := uint16(startVLANPort + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		CreateDUTSubinterface(t, vrfBatch, d, dut, dutPort, index, vlanID, dutIPsV4[i], dutIPsV6[i])
	}
}

// CreateDUTSubinterface creates one VLAN-tagged sub-interface on a DUT port.
func CreateDUTSubinterface(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root,
	dut *ondatra.DUTDevice, dutPort *ondatra.Port,
	index uint32, vlanID uint16, ipv4Addr, ipv6Addr string) {
	t.Helper()
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(ipv4Addr)
	a4.PrefixLength = ygot.Uint8(uint8(IPv4IntfMask))
	if MTU > 0 {
		s4.Mtu = ygot.Uint16(MTU)
	}
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6Addr)
	a6.PrefixLength = ygot.Uint8(uint8(IPv6IntfMask))
	if MTU > 0 {
		s6.Mtu = ygot.Uint32(uint32(MTU))
	}
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), s)

	assignInterfaceToDefaultNI(t, vrfBatch, dut, dutPort.Name(), index)
}

// assignInterfaceToDefaultNI assigns a subinterface on the port to the DEFAULT network instance.
func assignInterfaceToDefaultNI(t *testing.T, vrfBatch *gnmi.SetBatch, dut *ondatra.DUTDevice, intfName string, subintIndex uint32) {
	t.Helper()
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		defaultNI := deviations.DefaultNetworkInstance(dut)
		netInst := &oc.NetworkInstance{Name: ygot.String(defaultNI)}
		id := fmt.Sprintf("%s.%d", intfName, subintIndex)
		netInstIntf, err := netInst.NewInterface(id)
		if err != nil {
			t.Fatalf("failed to create default NI interface ref: %v", err)
		}
		netInstIntf.Interface = ygot.String(intfName)
		netInstIntf.Subinterface = ygot.Uint32(subintIndex)
		netInstIntf.Id = ygot.String(id)
		gnmi.BatchUpdate(vrfBatch, gnmi.OC().NetworkInstance(defaultNI).Interface(id).Config(), netInstIntf)
	}
}

// setDUTSubinterfaceAdminState configures the admin state of select VLAN subinterfaces on the DUT.
func setDUTSubinterfaceAdminState(t *testing.T, dut *ondatra.DUTDevice, portName string, startIdx, endIdx int, enabled bool) {
	t.Helper()
	t.Logf("Setting DUT subinterfaces %d-%d on port %s to enabled=%t", startIdx, endIdx, portName, enabled)
	sb := &gnmi.SetBatch{}
	for i := startIdx; i <= endIdx; i++ {
		p := gnmi.OC().Interface(portName).Subinterface(uint32(i))
		gnmi.BatchUpdate(sb, p.Enabled().Config(), enabled)
	}
	sb.Set(t, dut)
}

// verifySubinterfaceStatus verifies that the operational status of select VLAN subinterfaces has reached the target status.
func verifySubinterfaceStatus(t *testing.T, dut *ondatra.DUTDevice, portName string, startIdx, endIdx int, enabled bool) {
	t.Helper()
	wantStatus := oc.Interface_OperStatus_UP
	if !enabled {
		wantStatus = oc.Interface_OperStatus_DOWN
	}
	t.Logf("Verifying subinterfaces %d-%d on %s reach oper-status %s", startIdx, endIdx, portName, wantStatus)
	start := time.Now()
	for i := startIdx; i <= endIdx; i++ {
		statePath := gnmi.OC().Interface(portName).Subinterface(uint32(i)).OperStatus().State()
		subStart := time.Now()
		_, ok := gnmi.Watch(t, dut, statePath, 2*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			if v, ok := val.Val(); ok {
				return v == wantStatus
			}
			return false
		}).Await(t)
		if !ok {
			t.Errorf("Subinterface %s:%d oper status did not reach %s after waiting %v", portName, i, wantStatus, time.Since(subStart))
		} else {
			t.Logf("Subinterface %s:%d reached oper-status %s in %v", portName, i, wantStatus, time.Since(subStart))
		}
	}
	t.Logf("Verifying subinterfaces %d-%d on %s completed in %v", startIdx, endIdx, portName, time.Since(start))
}

// ConfigureHardwareInit pushes platform-specific hardware init configs.
func ConfigureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequired := false
	if dut.Vendor() == ondatra.ARISTA {
		rebootRequired = ConfigureTcam(t, dut)
	} else if dut.Vendor() == ondatra.NOKIA {
		rebootRequired = EnableSecondaryDefaultLookup(t, dut)
	}
	if rebootRequired {
		RebootChassis(t, dut)
	}
}

// ConfigureTcam pushes Arista-specific hardware init configs for TCAM to allocate enough space
// for routes and optimize FIB and counters.
func ConfigureTcam(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	hardwareVrfCfg := NewDUTHardwareInit(t, dut, FeatureVrfSelectionExtended)
	hardwarePfCfg := NewDUTHardwareInit(t, dut, FeatureOptimizeFIBAndCounters)
	if hardwareVrfCfg != "" && hardwarePfCfg != "" {
		PushDUTHardwareInitConfig(t, dut, hardwareVrfCfg)
		PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
		// Save the configurations before rebooting the chassis.
		helpers.GnmiCLIConfig(t, dut, "write memory")
		return true
	}
	return false
}

// EnableSecondaryDefaultLookup pushes Nokia specific hardware configuration to enable secondary
// default lookup. It is required for decap flows due to the fact that decapsulated packets
// are forwarded to the default VRF by the static route 0.0.0.0/0 from the encap VRF.
func EnableSecondaryDefaultLookup(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	cliConfig := NewDUTHardwareInit(t, dut, FeatureSecondaryDefaultLookup)
	if cliConfig != "" {
		PushDUTHardwareInitConfig(t, dut, cliConfig)
		return true
	}
	return false
}

// CreateGRIBIScaleVRFs creates all non-default VRF network-instances plus the DEFAULT instance.  Uses deviations.DefaultNetworkInstance for the correct name.
func CreateGRIBIScaleVRFs(t *testing.T, dut *ondatra.DUTDevice, vrfBatch *gnmi.SetBatch, numEncapVRFs int) {
	t.Helper()
	droot := new(oc.Root)

	// DEFAULT NI.
	defaultNI := deviations.DefaultNetworkInstance(dut)
	ni := droot.GetOrCreateNetworkInstance(defaultNI)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().NetworkInstance(defaultNI).Config(), ni)

	// All non-default VRFs.
	encapVRFNames := BuildEncapVRFNames(numEncapVRFs)
	allNonDefaultVRFs := BuildAllNonDefaultVRFs(encapVRFNames)
	for _, vrf := range allNonDefaultVRFs {
		ni := droot.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.BatchUpdate(vrfBatch, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
	vrfBatch.Set(t, dut)
}

// CreateOTGConfig builds and returns the OTG config for both ATE ports.
func CreateOTGConfig(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, params ScaleParams) (gosnappi.Config, []string) {
	t.Helper()
	ateConfig := gosnappi.NewConfig()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	ateConfig.Ports().Add().SetName(ap1.ID())
	ateConfig.Ports().Add().SetName(ap2.ID())

	// Base devices for each port.
	vlanIDPort1 := uint16(0)
	vlanIDPort2 := uint16(0)
	if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		vlanIDPort2 = NoMixVlanIDBase
	}
	CreateATEDevice(t, ateConfig, ap1, vlanIDPort1, atePort1Attr.Name, atePort1Attr.MAC, dutPort1Attr.IPv4, atePort1Attr.IPv4, dutPort1Attr.IPv6, atePort1Attr.IPv6)
	CreateATEDevice(t, ateConfig, ap2, vlanIDPort2, atePort2Attr.Name, atePort2Attr.MAC, dutPort2Attr.IPv4, atePort2Attr.IPv4, dutPort2Attr.IPv6, atePort2Attr.IPv6)

	// VLAN sub-interfaces.
	ifNames := []string{atePort1Attr.Name}
	port2SubintNames := MustConfigureATESubinterfaces(t, ateConfig, ap2, dut, atePort2Attr.Name, atePort2Attr.MAC, DUTPort2IPv4Start, ATEPort2IPv4Start, DUTPort2IPv6Start, ATEPort2IPv6Start, StartVLANPort2, params.NumPort2VLANs)
	ifNames = append(ifNames, port2SubintNames...)

	return ateConfig, ifNames
}

// configureOTG instantiates the ATE, creates the OTG config, pushes the config to the ATE,
// starts protocols, validates neighbor resolution, and returns the ATE device, config, and DUT MAC address.
func configureOTG(t *testing.T, dut *ondatra.DUTDevice, params ScaleParams) (*ondatra.ATEDevice, gosnappi.Config, string) {
	t.Helper()
	ate := ondatra.ATE(t, "ate")

	t.Log("Configuring ATE topology")
	ateConfig, interfaceNamesList := CreateOTGConfig(t, ate, dut, params)
	ate.OTG().PushConfig(t, ateConfig)
	time.Sleep(1 * time.Minute)
	ate.OTG().StartProtocols(t)
	time.Sleep(1 * time.Minute)

	t.Log("Validating ARP resolution for IPv4 and IPv6 interfaces")
	IsIPv4InterfaceARPresolved(t, ate, AddressFamilyParams{InterfaceNames: interfaceNamesList})
	IsIPv6InterfaceARPresolved(t, ate, AddressFamilyParams{InterfaceNames: interfaceNamesList})

	// Fetch MAC address for port1.
	// The ATE needs to resolve the MAC address of the DUT to send traffic to it.
	intfName := atePort1Attr.Name + ".Eth"
	dutIngressMAC := GetDUTMACAddress(t, ate, intfName, DUTPort1IPv4)

	return ate, ateConfig, dutIngressMAC
}

// CreateATEDevice creates a single ATE device with Ethernet, optional VLAN, IPv4 and IPv6 configuration.
func CreateATEDevice(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, vlanID uint16, name, mac, dutIPv4, ateIPv4, dutIPv6, ateIPv6 string) {
	t.Helper()
	dev := ateConfig.Devices().Add().SetName(name + ".Dev")
	eth := dev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
	eth.Connection().SetPortName(atePort.ID())
	if MTU > 0 {
		eth.SetMtu(uint32(MTU))
	}
	if vlanID > 0 {
		eth.Vlans().Add().SetName(name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(IPv4IntfMask))
	eth.Ipv6Addresses().Add().SetName(name + ".IPv6").SetAddress(ateIPv6).SetGateway(dutIPv6).SetPrefix(uint32(IPv6IntfMask))
}

// MustConfigureATESubinterfaces creates VLAN sub-interfaces on an ATE port and returns the device name list.
func MustConfigureATESubinterfaces(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice, name, mac, dutPfxV4, atePfxV4, dutPfxV6, atePfxV6 string, startVLAN, count int) []string {
	t.Helper()
	dutV4, err := iputil.GenerateIPsWithStep(dutPfxV4, count, PortIPv4Step)
	if err != nil {
		t.Fatalf("generate DUT IPv4s: %v", err)
	}
	ateV4, err := iputil.GenerateIPsWithStep(atePfxV4, count, PortIPv4Step)
	if err != nil {
		t.Fatalf("generate ATE IPv4s: %v", err)
	}
	dutV6, err := iputil.GenerateIPv6sWithStep(dutPfxV6, count, PortIPv6Step)
	if err != nil {
		t.Fatalf("generate DUT IPv6s: %v", err)
	}
	ateV6, err := iputil.GenerateIPv6sWithStep(atePfxV6, count, PortIPv6Step)
	if err != nil {
		t.Fatalf("generate ATE IPv6s: %v", err)
	}
	var names []string
	for i := range count {
		vlanID := uint16(startVLAN + i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID++
		}
		devName := fmt.Sprintf("%s-%d", name, i)
		incMAC, err := iputil.IncrementMAC(mac, i+1)
		if err != nil {
			t.Fatalf("increment MAC: %v", err)
		}
		CreateATEDevice(t, ateConfig, atePort, vlanID, devName, incMAC, dutV4[i], ateV4[i], dutV6[i], ateV6[i])
		names = append(names, devName)
	}
	return names
}

// NewGRIBIClient creates, starts, and returns a gRIBI client for the DUT.
func NewGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *gribi.Client {
	t.Helper()
	c := &gribi.Client{DUT: dut, FIBACK: true, Persistence: true}
	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI connection could not be established: %v", err)
	}
	c.BecomeLeader(t)
	return c
}

// BatchModify pushes entries to the DUT in chunks of gribiBatchSize over the provided gSession.
// If wTime > 0, it awaits FIB ACK convergence after each batch. If wTime <= 0, per-batch await is skipped.
func BatchModify(t *testing.T, gSession *gribi.Client, ctx context.Context, entries []fluent.GRIBIEntry,
	gribiBatchSize int, wTime time.Duration, pauseBetweenBatches time.Duration) {
	t.Helper()
	if gribiBatchSize <= 0 {
		gribiBatchSize = DefaultGRIBIBatchSize
	}
	totalEntries := len(entries)
	if totalEntries == 0 {
		return
	}

	totalBatches := (totalEntries + gribiBatchSize - 1) / gribiBatchSize
	for i := 0; i < totalEntries; i += gribiBatchSize {
		batchNum := (i / gribiBatchSize) + 1
		end := i + gribiBatchSize
		if end > totalEntries {
			end = totalEntries
		}
		t.Logf("Programming gRIBI batch %d/%d (%d entries, total: %d/%d entries)...", batchNum, totalBatches, end-i, end, totalEntries)
		batchStart := time.Now()
		gSession.AddEntries(t, entries[i:end], nil)
		if wTime > 0 {
			if err := gSession.AwaitTimeout(ctx, t, wTime); err != nil {
				t.Fatalf("gRIBI batch programming timeout on batch %d/%d after %v: %v", batchNum, totalBatches, time.Since(batchStart), err)
			}
			t.Logf("Completed gRIBI batch %d/%d in %v", batchNum, totalBatches, time.Since(batchStart))
		}
		if pauseBetweenBatches > 0 {
			time.Sleep(pauseBetweenBatches)
		}
	}
}

// AwaitUnreferencedFIBAcks performs final FIB-ACK verification for unreferenced entries on devices requiring delayed ACK handling.
func AwaitUnreferencedFIBAcks(t *testing.T, gSession *gribi.Client, dut *ondatra.DUTDevice, wTime time.Duration) {
	t.Helper()
	if deviations.UnreferencedAftFibAckUnsupported(dut) {
		t.Log("UnreferencedAftFibAckUnsupported: performing final FIB-ACK await for unreferenced entries...")
		start := time.Now()
		if err := gSession.AwaitTimeout(context.Background(), t, wTime); err != nil {
			t.Fatalf("gRIBI FIB-ACK verification timeout after %v: %v", time.Since(start), err)
		}
		t.Logf("Completed FIB-ACK verification in %v", time.Since(start))
	}
}

// ProgramAndVerifyGribiEntries pushes NH, NHG, and IP entries in batches, validates operation results,
// executes optional verification callbacks, and closes the session(s).
func ProgramAndVerifyGribiEntries(
	t *testing.T,
	dut *ondatra.DUTDevice,
	ctx context.Context,
	nhEntries []fluent.GRIBIEntry,
	nhgEntries []fluent.GRIBIEntry,
	ipEntries []fluent.GRIBIEntry,
	params ScaleParams,
	verifyFunc func(gSession *gribi.Client),
) {
	t.Helper()
	gSession := NewGRIBIClient(t, dut)

	// Ideally we want to program all entries without splitting them by type and wait for FIB-ACKs after every batch.
	// However, if the device does not send FIB-ACKs for unreferenced entries, awaiting for FIB-ACKs will cause a timeout.
	// In this case, we split the entries into separate batches for NH, NHG, and IP entries.
	// At the end, we await FIB-ACKs for all unreferenced entries.
	noPauseBetweenBatches := 0 * time.Second
	if deviations.UnreferencedAftFibAckUnsupported(dut) {
		noWaitTimeForFIBAck := 0 * time.Second
		pauseBetweenBatches := 200 * time.Millisecond
		if len(nhEntries) > 0 {
			t.Logf("Programming %d NH entries...", len(nhEntries))
			// NH entries are programmed without waiting for FIB-ACKs since they are referenced by NHG
			// entries which are not programmed yet.
			BatchModify(t, gSession, ctx, nhEntries, params.GRIBIBatchSize, noWaitTimeForFIBAck, pauseBetweenBatches)
		}
		if len(nhgEntries) > 0 {
			// Waiting for FIB-ACKs for NHG entries is not possible since some next hops may not be
			// referenced yet.
			t.Logf("Programming %d NHG entries...", len(nhgEntries))
			BatchModify(t, gSession, ctx, nhgEntries, params.GRIBIBatchSize, noWaitTimeForFIBAck, pauseBetweenBatches)
		}
		if len(ipEntries) > 0 {
			t.Logf("Programming %d IP entries...", len(ipEntries))
			BatchModify(t, gSession, ctx, ipEntries, params.GRIBIBatchSize, params.GRIBIBatchTimeout, noPauseBetweenBatches)
		}
		AwaitUnreferencedFIBAcks(t, gSession, dut, params.GRIBIBatchTimeout)
	} else {
		// Program all entries without paying attention to unreferenced entries and wait for FIB-ACKs after
		// every batch.
		allEntries := append(append(append([]fluent.GRIBIEntry{}, nhEntries...), nhgEntries...), ipEntries...)
		if len(allEntries) > 0 {
			BatchModify(t, gSession, ctx, allEntries, params.GRIBIBatchSize, params.GRIBIBatchTimeout, noPauseBetweenBatches)
		}
	}

	// Parse results, check for errors and log failures.
	ValidateGRIBIResults(t, gSession)

	if verifyFunc != nil {
		verifyFunc(gSession)
	}

	gSession.Close(t)
}

// buildNHGs constructs next hop groups using the provided load balancing and weight parameters,
// assigning member next hops round-robin starting from baseNHID across numNH next hops.
// If backupNHG is non-zero, it sets the backup next-hop group on each generated group.
func buildNHGs(networkInstance string, baseNHGID uint64, numNHG int, baseNHID uint64, numNH int, lbParams []NHGLoadBalancingParams, weightParams []NHGWeightParams, backupNHG uint64) []fluent.GRIBIEntry {
	if numNHG <= 0 || numNH <= 0 {
		return nil
	}
	groups := make([]fluent.GRIBIEntry, 0, numNHG)
	iter := NewNHGBucketIterator(numNHG, lbParams, weightParams, numNH)

	nhOffset := 0
	for i := 0; i < numNHG; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(networkInstance).WithID(baseNHGID + uint64(i))
		if backupNHG != 0 {
			nhg.WithBackupNHG(backupNHG)
		}
		weights := iter.Next()
		for j, w := range weights {
			nhID := baseNHID + uint64((nhOffset+j)%numNH)
			nhg.AddNextHop(nhID, w)
		}
		nhOffset += len(weights)
		groups = append(groups, nhg)
	}
	return groups
}

// splitDefaultVRFPrimaryBackup returns the number of next hops and next hop groups
// for each partition (primary and backup) in the Default VRF.
func splitDefaultVRFPrimaryBackup(params ScaleParams) (int, int) {
	return max(params.NumDefaultNH/2, 1), max(params.NumDefaultNHG/2, 1)
}

// BuildDefaultVRF generates NHs, NHGs, and IPv4 entries for the default VRF.
func BuildDefaultVRF(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, params ScaleParams) ([]string, []string) {
	t.Helper()
	wantPrefixes := make(map[string][]string)
	nhBase, nhgBase := NHBaseDefault, NHGBaseDefault
	atePort2Ips, err := iputil.GenerateIPsWithStep(ATEPort2IPv4Start, params.NumPort2VLANs, PortIPv4Step)
	if err != nil {
		t.Fatalf("ConfigureOTG: generate ATE port2 IPs: %v", err)
	}
	prefixHosts, err := iputil.GenerateIPsWithStep(IPv4PrefixStartAddress, params.NumDefaultIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildDefaultVRF: generate prefix IPs: %v", err)
	}

	nhEntries := []fluent.GRIBIEntry{}
	nhgEntries := []fluent.GRIBIEntry{}
	ipv4Entries := []fluent.GRIBIEntry{}

	numNHPart, numNHGPart := splitDefaultVRFPrimaryBackup(params)
	numIPv4Part := max(params.NumDefaultIPv4/2, 1)
	nhgBaseBackup := nhgBase + uint64(numNHGPart)

	// Split the VLANs into primary and backup sets.
	primaryVLANs := max(params.NumPort2VLANs*ShutdownVLANPct/100, 1)
	backupVLANs := params.NumPort2VLANs - primaryVLANs

	// Setup Set 1 next-hops (resolving ONLY to first part of VLANs up to primaryVLANs)
	for i := 0; i < numNHPart; i++ {
		vlanIdx := i % primaryVLANs
		nhEntry := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhBase + uint64(i)).WithIPAddress(atePort2Ips[vlanIdx])
		nhEntries = append(nhEntries, nhEntry)
	}

	// Setup Set 2 next-hops (resolving ONLY to backup VLANs)
	nhBaseBackup := nhBase + uint64(numNHPart)
	for i := 0; i < numNHPart; i++ {
		vlanIdx := 0
		if backupVLANs > 0 {
			vlanIdx = primaryVLANs + (i % backupVLANs)
		} else {
			vlanIdx = i % primaryVLANs
		}
		nhEntry := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhBaseBackup + uint64(i)).WithIPAddress(atePort2Ips[vlanIdx])
		nhEntries = append(nhEntries, nhEntry)
	}

	nhgEntries = append(nhgEntries, buildNHGs(defaultVRF, nhgBase, numNHGPart, nhBase, numNHPart, params.DefaultNHGLoadBalance, params.DefaultNHGWeight, 0)...)
	nhgEntries = append(nhgEntries, buildNHGs(defaultVRF, nhgBaseBackup, numNHGPart, nhBaseBackup, numNHPart, params.DefaultNHGLoadBalance, params.DefaultNHGWeight, 0)...)

	// Split the prefixes into primary and backup sets to match the VLAN split.
	// The primary prefixes will be used by transit VRF (TE_VRF_111) and backup prefixes will be used by
	// repaired VRF (TE_VRF_222).
	primaryPrefixes := make([]string, numIPv4Part)
	backupPrefixes := make([]string, numIPv4Part)

	// Setup Set 1 default prefixes (Primary)
	for i := 0; i < numIPv4Part; i++ {
		primaryPrefixes[i] = prefixHosts[i]
		ipv4Entries = append(ipv4Entries, fluent.IPv4Entry().WithNetworkInstance(defaultVRF).
			WithPrefix(fmt.Sprintf("%s/%d", primaryPrefixes[i], IPv4HostMask)).
			WithNextHopGroup(nhgBase+uint64(i%numNHGPart)).WithNextHopGroupNetworkInstance(defaultVRF))
	}

	// Setup Set 2 default prefixes (Backup)
	for i := 0; i < numIPv4Part; i++ {
		prefixIdx := numIPv4Part + i
		if prefixIdx >= len(prefixHosts) {
			prefixIdx = i % len(prefixHosts)
		}
		backupPrefixes[i] = prefixHosts[prefixIdx]
		ipv4Entries = append(ipv4Entries, fluent.IPv4Entry().WithNetworkInstance(defaultVRF).
			WithPrefix(fmt.Sprintf("%s/%d", backupPrefixes[i], IPv4HostMask)).
			WithNextHopGroup(nhgBaseBackup+uint64(i%numNHGPart)).WithNextHopGroupNetworkInstance(defaultVRF))
	}

	t.Logf("BuildDefaultVRF: %d NHs (shared), %d NHGs (shared), %d IPv4 entries (shared)", params.NumDefaultNH, params.NumDefaultNHG, params.NumDefaultIPv4)
	// Validate only the first and last prefixes to save time.
	wantPrefixes[defaultVRF] = append(wantPrefixes[defaultVRF], fmt.Sprintf("%s/%d", prefixHosts[0], IPv4HostMask))
	wantPrefixes[defaultVRF] = append(wantPrefixes[defaultVRF], fmt.Sprintf("%s/%d", prefixHosts[len(prefixHosts)-1], IPv4HostMask))
	ProgramAndVerifyGribiEntries(t, dut, ctx, nhEntries, nhgEntries, ipv4Entries, params, func(gSession *gribi.Client) {
		VerifyFIBProgrammed(t, gSession, wantPrefixes, nil)
	})
	return primaryPrefixes, backupPrefixes
}

// BuildStaticGroups generates entries for the two static NHGs (S1 → REPAIR_VRF, S2 → decap DEFAULT).
func BuildStaticGroups(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, params ScaleParams) (uint64, uint64) {
	t.Helper()
	s1NHG, s2NHG := StaticS1NHG, StaticS2NHG
	s1NH, _ := gribi.NHEntry(s1NHG, "VRFOnly", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: RepairVRFStr})
	s1NHGEntry, _ := gribi.NHGEntry(s1NHG, map[uint64]uint64{s1NHG: 1}, defaultVRF, fluent.InstalledInFIB)
	s2NH, _ := gribi.NHEntry(s2NHG, "Decap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: defaultVRF})
	s2NHGEntry, _ := gribi.NHGEntry(s2NHG, map[uint64]uint64{s2NHG: 1}, defaultVRF, fluent.InstalledInFIB)
	t.Logf("BuildStaticGroups: S1 NHG=%d (→REPAIR_VRF), S2 NHG=%d (decap→DEFAULT)", s1NHG, s2NHG)
	ProgramAndVerifyGribiEntries(t, dut, ctx, []fluent.GRIBIEntry{s1NH, s2NH}, []fluent.GRIBIEntry{s1NHGEntry, s2NHGEntry}, nil, params, nil)
	return s1NHG, s2NHG
}

// splitTransitVRFPrimaryBackup returns the (primaryNH, primaryNHG, backupNH, backupNHG) counts
// for the Transit VRFs (TE_VRF_111 and TE_VRF_222).
func splitTransitVRFPrimaryBackup(params ScaleParams) (primaryNH, primaryNHG, backupNH, backupNHG int) {
	primaryNH = max(params.NumTransitNH/2, 1)
	primaryNHG = max(params.NumTransitNHG/2, 1)
	backupNH = max(params.NumTransitNH-primaryNH, 0)
	backupNHG = max(params.NumTransitNHG-primaryNHG, 0)
	return primaryNH, primaryNHG, backupNH, backupNHG
}

// BuildTransitVRFs generates entries for TE_VRF_111 and TE_VRF_222.
func BuildTransitVRFs(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, primaryDefaultPrefixes, backupDefaultPrefixes []string, s1NHG, s2NHG uint64, params ScaleParams) {
	t.Helper()
	validatePrefixesV4 := make(map[string][]string)
	nhEntries := make([]fluent.GRIBIEntry, 0, params.NumTransitNH)
	nhgEntries := make([]fluent.GRIBIEntry, 0, params.NumTransitNHG)
	ipv4Entries := make([]fluent.GRIBIEntry, 0, 2*params.NumTransitIPv4)

	buildTransitVRF := func(vrfName string, prefixStart string, defaultPrefixes []string, baseNHId uint64, nhCount int, baseNHGId uint64, nhgCount int, backupNHG uint64) {
		if nhCount == 0 || nhgCount == 0 {
			return
		}
		// Create next hops pointing to default network instance.
		for k := 0; k < nhCount; k++ {
			nhEntries = append(nhEntries, fluent.NextHopEntry().WithNetworkInstance(defaultVRF).
				WithIndex(baseNHId+uint64(k)).
				WithIPAddress(defaultPrefixes[k%len(defaultPrefixes)]))
		}

		// Create next hop groups referencing default network instance NHs.
		nhgEntries = append(nhgEntries, buildNHGs(
			defaultVRF,
			baseNHGId,
			nhgCount,
			baseNHId,
			nhCount,
			params.TransitNHGLoadBalance,
			params.TransitNHGWeight,
			backupNHG,
		)...)

		// Create IPv4 prefixes in the specific Transit VRF.
		vrfPrefixes, err := iputil.GenerateIPsWithStep(prefixStart, params.NumTransitIPv4, CommonPrefixStep)
		if err != nil {
			t.Fatalf("BuildTransitVRFs: generate %s prefixes: %v", vrfName, err)
		}
		for i, host := range vrfPrefixes {
			pfx := fmt.Sprintf("%s/%d", host, IPv4HostMask)
			ipv4Entries = append(ipv4Entries, fluent.IPv4Entry().WithNetworkInstance(vrfName).WithPrefix(pfx).
				WithNextHopGroup(baseNHGId+uint64(i%nhgCount)).WithNextHopGroupNetworkInstance(defaultVRF))
		}
		// Validate only the first prefix to save time.
		validatePrefixesV4[vrfName] = []string{fmt.Sprintf("%s/%d", vrfPrefixes[0], IPv4HostMask)}
	}

	nhPrimaryCount, nhgPrimaryCount, nhBackupCount, nhgBackupCount := splitTransitVRFPrimaryBackup(params)
	buildTransitVRF(TransitVRF111Str, TransitVRF111PrefixStart, primaryDefaultPrefixes,
		NHBaseTransit, nhPrimaryCount,
		NHGBaseTransit, nhgPrimaryCount,
		s1NHG)

	buildTransitVRF(TransitVRF222Str, TransitVRF222PrefixStart, backupDefaultPrefixes,
		NHBaseTransit+uint64(nhPrimaryCount), nhBackupCount,
		NHGBaseTransit+uint64(nhgPrimaryCount), nhgBackupCount,
		s2NHG)

	t.Logf("BuildTransitVRFs: %d NHs total, %d NHGs total", params.NumTransitNH, params.NumTransitNHG)
	t.Logf("BuildTransitVRFs: %d IPv4 entries each transit VRF", params.NumTransitIPv4)
	ProgramAndVerifyGribiEntries(t, dut, ctx, nhEntries, nhgEntries, ipv4Entries, params, func(gSession *gribi.Client) {
		VerifyFIBProgrammed(t, gSession, validatePrefixesV4, nil)
		VerifyHierarchicalResolution(t, gSession, dut, validatePrefixesV4)
	})
}

// Currently not all vendors support repair reencap into multiple tunnels.
// This condition should be removed once all vendors support it.
func repairReencapIntoMultipleBackupTunnels(dut *ondatra.DUTDevice) bool {
	return dut.Vendor() != ondatra.NOKIA
}

// BuildRepairVRF generates NH/NHG/IPv4 entries for REPAIR_VRF.
func BuildRepairVRF(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, s2NHG uint64, params ScaleParams) {
	t.Helper()
	tunnelDsts, err := iputil.GenerateIPsWithStep(TransitVRF222PrefixStart, params.NumRepairNHG*2, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildRepairVRF: generate tunnel dsts: %v", err)
	}

	var nhEntries []fluent.GRIBIEntry
	var nhgEntries []fluent.GRIBIEntry
	nhIdx := uint64(0)
	for i := 0; i < params.NumRepairNHG; i++ {
		if i < params.NumRepairNHG/2 || !repairReencapIntoMultipleBackupTunnels(dut) {
			// Repair NHG points to a single NH.
			nhEntry, _ := gribi.NHEntry(NHBaseRepair+nhIdx, "DecapEncap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx], VrfName: TransitVRF222Str})
			nhgEntry, _ := gribi.NHGEntry(NHGBaseRepair+uint64(i), map[uint64]uint64{NHBaseRepair + nhIdx: 1}, defaultVRF, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: s2NHG})
			nhEntries = append(nhEntries, nhEntry)
			nhgEntries = append(nhgEntries, nhgEntry)
			nhIdx++
		} else {
			// Repair NHG points to two NHs.
			nh0, _ := gribi.NHEntry(NHBaseRepair+nhIdx, "DecapEncap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx], VrfName: TransitVRF222Str})
			nh1, _ := gribi.NHEntry(NHBaseRepair+nhIdx+1, "DecapEncap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx+1], VrfName: TransitVRF222Str})
			nhgEntry, _ := gribi.NHGEntry(NHGBaseRepair+uint64(i), map[uint64]uint64{NHBaseRepair + nhIdx: 1, NHBaseRepair + nhIdx + 1: 1}, defaultVRF, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: s2NHG})
			nhEntries = append(nhEntries, nh0, nh1)
			nhgEntries = append(nhgEntries, nhgEntry)
			nhIdx += 2
		}
	}

	// The repair IP prefixes should match exactly the transit IP prefixes.
	repairPrefixes, err := iputil.GenerateIPsWithStep(TransitVRF111PrefixStart, params.NumRepairIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildRepairVRF: generate repair prefixes: %v", err)
	}
	var ipv4Entries []fluent.GRIBIEntry
	wantPrefixes := make(map[string][]string)
	for i, host := range repairPrefixes {
		pfx := fmt.Sprintf("%s/%d", host, IPv4HostMask)
		ipv4Entries = append(ipv4Entries, fluent.IPv4Entry().WithNetworkInstance(RepairVRFStr).WithPrefix(pfx).WithNextHopGroup(NHGBaseRepair+uint64(i%params.NumRepairNHG)).WithNextHopGroupNetworkInstance(defaultVRF))
	}
	wantPrefixes[RepairVRFStr] = append(wantPrefixes[RepairVRFStr], fmt.Sprintf("%s/%d", repairPrefixes[0], IPv4HostMask))
	wantPrefixes[RepairVRFStr] = append(wantPrefixes[RepairVRFStr], fmt.Sprintf("%s/%d", repairPrefixes[len(repairPrefixes)-1], IPv4HostMask))

	t.Logf("BuildRepairVRF: %d NHGs (%d NHs), %d IPv4 entries", params.NumRepairNHG, int(nhIdx), params.NumRepairIPv4)
	ProgramAndVerifyGribiEntries(t, dut, ctx, nhEntries, nhgEntries, ipv4Entries, params, func(gSession *gribi.Client) {
		VerifyFIBProgrammed(t, gSession, wantPrefixes, nil)
	})
}

// BuildEncapVRFs generates all encap NH/NHG/IPv4/IPv6 entries and programs each VRF individually.
func BuildEncapVRFs(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, monitorHWUtilization bool, params ScaleParams) {
	t.Helper()

	totalEncapNH := params.NumEncapVRFs * params.NumEncapNHPerVRF
	numOfTunnelsToUse := min(totalEncapNH, params.NumTransitIPv4)
	if numOfTunnelsToUse <= 0 {
		t.Fatalf("BuildEncapVRFs: numOfTunnelsToUse (%d) must be greater than 0 (totalEncapNH=%d, NumTransitIPv4=%d)", numOfTunnelsToUse, totalEncapNH, params.NumTransitIPv4)
	}
	tunnelDsts, err := iputil.GenerateIPsWithStep(TransitVRF111PrefixStart, numOfTunnelsToUse, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildEncapVRFs: generate encap NH tunnel dsts: %v", err)
	}

	fallbackNHID := uint64(NHBaseEncap)
	fallbackNH, _ := gribi.NHEntry(fallbackNHID, "VRFOnly", defaultVRF, fluent.InstalledInFIB,
		&gribi.NHOptions{VrfName: defaultVRF})

	fallbackNHGID := uint64(NHGBaseEncap)
	fallbackNHG, _ := gribi.NHGEntry(fallbackNHGID, map[uint64]uint64{fallbackNHID: 1}, defaultVRF, fluent.InstalledInFIB)

	nhBaseIdx := fallbackNHID + 1
	nhgBaseIdx := fallbackNHGID + 1

	encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)

	buildEncapVRF := func(vi int, vrf string) {
		nhOffset := vi * params.NumEncapNHPerVRF
		nhgOffset := vi * params.NumEncapNHGPerVRF
		nhEntries := []fluent.GRIBIEntry{}
		nhgEntries := []fluent.GRIBIEntry{}
		ipEntries := []fluent.GRIBIEntry{}
		wantPrefixesV4 := make(map[string][]string)
		wantPrefixesV6 := make(map[string][]string)

		if vi == 0 {
			nhEntries = append(nhEntries, fallbackNH)
			nhgEntries = append(nhgEntries, fallbackNHG)
		}

		// Create NextHops assigned ONLY to this VRF
		for i := 0; i < params.NumEncapNHPerVRF; i++ {
			globalNHIdx := nhOffset + i
			nhEntry, _ := gribi.NHEntry(nhBaseIdx+uint64(globalNHIdx), "Encap", defaultVRF, fluent.InstalledInFIB,
				&gribi.NHOptions{Src: IPv4OuterSrc111, Dest: tunnelDsts[globalNHIdx%numOfTunnelsToUse], VrfName: TransitVRF111Str})
			nhEntries = append(nhEntries, nhEntry)
		}

		// Create NextHopGroups for this VRF, pointing ONLY to NHs assigned to this VRF
		nhgEntries = append(nhgEntries, buildNHGs(
			defaultVRF,
			nhgBaseIdx+uint64(nhgOffset),
			params.NumEncapNHGPerVRF,
			nhBaseIdx+uint64(nhOffset),
			params.NumEncapNHPerVRF,
			params.EncapNHGLoadBalance,
			params.EncapNHGWeight,
			0,
		)...)

		// Create IPv4 and IPv6 routes for the current VRF
		v4Prefixes, v4Err := iputil.GenerateIPsWithStep(fmt.Sprintf("200.%d.0.1", vi), params.NumEncapIPv4PerVRF, CommonPrefixStep)
		if v4Err != nil {
			t.Fatalf("Failed to generate IPv4 prefixes for VRF %s (vi=%d): %v", vrf, vi, v4Err)
		}
		for i, host := range v4Prefixes {
			v4NHGID := nhgBaseIdx + uint64(nhgOffset+(i%params.NumEncapNHGPerVRF))
			ipEntries = append(ipEntries, fluent.IPv4Entry().WithNetworkInstance(vrf).
				WithPrefix(fmt.Sprintf("%s/%d", host, IPv4HostMask)).
				WithNextHopGroup(v4NHGID).WithNextHopGroupNetworkInstance(defaultVRF))
		}
		wantPrefixesV4[vrf] = append(wantPrefixesV4[vrf], fmt.Sprintf("%s/%d", v4Prefixes[0], IPv4HostMask))
		wantPrefixesV4[vrf] = append(wantPrefixesV4[vrf], fmt.Sprintf("%s/%d", v4Prefixes[len(v4Prefixes)-1], IPv4HostMask))

		v6Prefixes, v6Err := iputil.GenerateIPv6sWithStep(fmt.Sprintf("2001:db8:%x::1", vi), params.NumEncapIPv6PerVRF, CommonIPv6PrefixStep)
		if v6Err != nil {
			t.Fatalf("Failed to generate IPv6 prefixes for VRF %s (vi=%d): %v", vrf, vi, v6Err)
		}
		for i, pfx := range v6Prefixes {
			v6NHGID := nhgBaseIdx + uint64(nhgOffset+(i%params.NumEncapNHGPerVRF))
			ipEntries = append(ipEntries, fluent.IPv6Entry().WithNetworkInstance(vrf).
				WithPrefix(fmt.Sprintf("%s/%d", pfx, IPv6HostMask)).
				WithNextHopGroup(v6NHGID).WithNextHopGroupNetworkInstance(defaultVRF))
		}
		wantPrefixesV6[vrf] = append(wantPrefixesV6[vrf], fmt.Sprintf("%s/%d", v6Prefixes[0], IPv6HostMask))
		wantPrefixesV6[vrf] = append(wantPrefixesV6[vrf], fmt.Sprintf("%s/%d", v6Prefixes[len(v6Prefixes)-1], IPv6HostMask))

		// Add Fallback route in Encap VRF
		ipEntries = append(ipEntries,
			fluent.IPv4Entry().WithNetworkInstance(vrf).
				WithPrefix("0.0.0.0/0").
				WithNextHopGroup(fallbackNHGID).WithNextHopGroupNetworkInstance(defaultVRF),
			fluent.IPv6Entry().WithNetworkInstance(vrf).
				WithPrefix("::/0").
				WithNextHopGroup(fallbackNHGID).WithNextHopGroupNetworkInstance(defaultVRF),
		)
		wantPrefixesV4[vrf] = append(wantPrefixesV4[vrf], "0.0.0.0/0")
		wantPrefixesV6[vrf] = append(wantPrefixesV6[vrf], "::/0")

		t.Logf("BuildEncapVRF: VRF %s (%d/%d): %d NHs, %d NHGs, %d IPv4, %d IPv6 entries", vrf, vi+1,
			len(encapVRFNames), params.NumEncapNHPerVRF, params.NumEncapNHGPerVRF, params.NumEncapIPv4PerVRF,
			params.NumEncapIPv6PerVRF)

		ProgramAndVerifyGribiEntries(t, dut, ctx, nhEntries, nhgEntries, ipEntries, params, func(gSession *gribi.Client) {
			VerifyFIBProgrammed(t, gSession, wantPrefixesV4, wantPrefixesV6)
		})

		LogHWUtilization(t, dut, monitorHWUtilization, fmt.Sprintf("Post-BuildEncapVRF-%s", vrf))
	}

	for vi, vrf := range encapVRFNames {
		buildEncapVRF(vi, vrf)
	}
}

// BuildDecapVRF generates all decap NH/NHG/IPv4 entries and programs them.
func BuildDecapVRF(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, params ScaleParams) {
	t.Helper()
	nhEntries := []fluent.GRIBIEntry{}
	nhgEntries := []fluent.GRIBIEntry{}
	ipEntries := []fluent.GRIBIEntry{}
	wantPrefixesV4 := make(map[string][]string)

	// DECAP_TE_VRF entries use variable prefix lengths — not host routes.
	for i := 0; i < params.NumDecapEntries; i++ {
		prefixLen := decapPrefixLens[i%len(decapPrefixLens)]
		pfx := fmt.Sprintf("203.%d.%d.0/%d", i/4, (i%4)*64, prefixLen)
		nhIdx := NHBaseDecap + uint64(i)
		nhgIdx := NHGBaseDecap + uint64(i)
		decapNH, _ := gribi.NHEntry(nhIdx, "Decap", defaultVRF, fluent.InstalledInFIB)
		decapNHG, _ := gribi.NHGEntry(nhgIdx, map[uint64]uint64{nhIdx: 1}, defaultVRF, fluent.InstalledInFIB)
		nhEntries = append(nhEntries, decapNH)
		nhgEntries = append(nhgEntries, decapNHG)
		ipEntries = append(ipEntries, fluent.IPv4Entry().WithNetworkInstance(DecapVRFStr).
			WithPrefix(pfx).WithNextHopGroup(nhgIdx).WithNextHopGroupNetworkInstance(defaultVRF))
		// Add first and last prefixes to wantPrefixesV4 for later verification.
		if i == 0 || i == params.NumDecapEntries-1 {
			wantPrefixesV4[DecapVRFStr] = append(wantPrefixesV4[DecapVRFStr], pfx)
		}
	}

	t.Logf("BuildDecapVRF: %d Decap entries", params.NumDecapEntries)
	ProgramAndVerifyGribiEntries(t, dut, ctx, nhEntries, nhgEntries, ipEntries, params, func(gSession *gribi.Client) {
		VerifyFIBProgrammed(t, gSession, wantPrefixesV4, nil)
	})
}

// VerifyFIBProgrammed checks that each prefix in wantPrefixesV4 and wantPrefixesV6 is FIB_PROGRAMMED in the gRIBI client results cache.
func VerifyFIBProgrammed(t *testing.T, c *gribi.Client, wantPrefixesV4 map[string][]string, wantPrefixesV6 map[string][]string) {
	t.Helper()
	res := c.Fluent(t).Results(t)

	verifyPrefixes := func(wantPrefixes map[string][]string, isIPv6 bool) {
		for vrf, prefixes := range wantPrefixes {
			wants := make([]*client.OpResult, 0, len(prefixes))
			for _, pfx := range prefixes {
				op := fluent.OperationResult().WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB)
				if isIPv6 {
					op = op.WithIPv6Operation(pfx)
				} else {
					op = op.WithIPv4Operation(pfx)
				}
				wants = append(wants, op.AsResult())
			}
			chk.HasResultsCache(t, res, wants, chk.IgnoreOperationID())
			t.Logf("VRF %s: %d prefixes confirmed FIB_PROGRAMMED (IPv6: %v)", vrf, len(prefixes), isIPv6)
		}
	}

	verifyPrefixes(wantPrefixesV4, false)
	verifyPrefixes(wantPrefixesV6, true)
}

// ProgramGRIBIRoutes programs all VRFs and routes (Default, Static, Repair, Transit, Decap, Encap) via gRIBI.
func ProgramGRIBIRoutes(t *testing.T, dut *ondatra.DUTDevice, defaultVRF string, params ScaleParams, monitorHWUtilization bool) {
	t.Helper()
	defer func() {
		if t.Failed() {
			LogHWUtilization(t, dut, monitorHWUtilization, "Failure-GRIBIRoutes")
		}
	}()
	ctx := context.Background()

	LogHWUtilization(t, dut, monitorHWUtilization, "Pre-BuildDefaultVRF")

	// DEFAULT VRF
	t.Log("Default VRF entries (A/B/C)")
	primaryDefaultPrefixes, backupDefaultPrefixes := BuildDefaultVRF(t, dut, ctx, defaultVRF, params)
	LogHWUtilization(t, dut, monitorHWUtilization, "Post-BuildDefaultVRF")

	// Static Groups
	t.Log("Static groups (S1/S2)")
	s1NHG, s2NHG := BuildStaticGroups(t, dut, ctx, defaultVRF, params)
	LogHWUtilization(t, dut, monitorHWUtilization, "Post-BuildStaticGroups")

	// Repair VRF
	t.Log("Repair VRF (F)")
	BuildRepairVRF(t, dut, ctx, defaultVRF, s2NHG, params)
	LogHWUtilization(t, dut, monitorHWUtilization, "Post-BuildRepairVRF")

	// Transit VRFs
	t.Log("Transit VRFs (D/E)")
	BuildTransitVRFs(t, dut, ctx, defaultVRF, primaryDefaultPrefixes, backupDefaultPrefixes, s1NHG, s2NHG, params)
	LogHWUtilization(t, dut, monitorHWUtilization, "Post-BuildTransitVRFs")

	// Decap VRF
	t.Log("Decap VRF (T4)")
	BuildDecapVRF(t, dut, ctx, defaultVRF, params)
	LogHWUtilization(t, dut, monitorHWUtilization, "Post-BuildDecapVRF")

	// Encap VRFs
	t.Log("Encap VRFs (T3)")
	BuildEncapVRFs(t, dut, ctx, defaultVRF, monitorHWUtilization, params)
}

// FlushGRIBIRoutes establishes a gRIBI session to the DUT, flushes all entries, and closes the session.
func FlushGRIBIRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	gSession := NewGRIBIClient(t, dut)
	t.Log("Flushing all entries from GRIBI session")
	gSession.FlushAll(t)
	gSession.Close(t)
}

// ValidateGRIBIResults validates the gRIBI results by looking for failures.
// It groups results by OperationID to verify that every AFT operation achieved FIB_PROGRAMMED.
// If an operation received RIB_PROGRAMMED but not FIB_PROGRAMMED, or experienced a server/client error,
// it is treated as a failure. It counts totals and logs the first 10 failures for each category.
func ValidateGRIBIResults(t *testing.T, gSession *gribi.Client) bool {
	t.Helper()

	results := gSession.Fluent(t).Results(t)

	// Group results by OperationID for AFT operations, and collect non-AFT operations separately.
	opMap := make(map[uint64][]*client.OpResult)
	var opOrder []uint64
	var otherResults []*client.OpResult

	for _, res := range results {
		if res.OperationID == 0 {
			otherResults = append(otherResults, res)
			continue
		}
		if _, exists := opMap[res.OperationID]; !exists {
			opOrder = append(opOrder, res.OperationID)
		}
		opMap[res.OperationID] = append(opMap[res.OperationID], res)
	}

	var nhTotalCount, nhgTotalCount, ipv4TotalCount, ipv6TotalCount, otherTotalCount int
	otherTotalCount = len(otherResults)

	getCategory := func(res *client.OpResult) string {
		if res.Details == nil {
			return "other"
		}
		if res.Details.NextHopIndex != 0 {
			return "nh"
		}
		if res.Details.NextHopGroupID != 0 {
			return "nhg"
		}
		if res.Details.IPv4Prefix != "" {
			return "ipv4"
		}
		if res.Details.IPv6Prefix != "" {
			return "ipv6"
		}
		return "other"
	}

	var nhFailures []*client.OpResult
	var nhgFailures []*client.OpResult
	var ipv4Failures []*client.OpResult
	var ipv6Failures []*client.OpResult
	var otherFailures []*client.OpResult

	var totalNHFailures, totalNHGFailures, totalIPv4Failures, totalIPv6Failures, totalOtherFailures int

	for _, opID := range opOrder {
		opResults := opMap[opID]
		var rep *client.OpResult
		for _, r := range opResults {
			if r.Details != nil {
				rep = r
				break
			}
		}
		if rep == nil {
			rep = opResults[len(opResults)-1]
		}

		cat := getCategory(rep)
		switch cat {
		case "nh":
			nhTotalCount++
		case "nhg":
			nhgTotalCount++
		case "ipv4":
			ipv4TotalCount++
		case "ipv6":
			ipv6TotalCount++
		default:
			otherTotalCount++
		}

		hasFIB := false
		var errRes *client.OpResult

		for _, r := range opResults {
			if r.ServerError != "" || r.ClientError != "" ||
				r.ProgrammingResult == gribipb.AFTResult_FIB_FAILED ||
				r.ProgrammingResult == gribipb.AFTResult_FAILED {
				errRes = r
				break
			}
			if r.ProgrammingResult == gribipb.AFTResult_FIB_PROGRAMMED {
				hasFIB = true
			}
		}

		if errRes != nil || !hasFIB {
			failRes := errRes
			if failRes == nil {
				failRes = &client.OpResult{
					Timestamp:         rep.Timestamp,
					Latency:           rep.Latency,
					OperationID:       rep.OperationID,
					ProgrammingResult: rep.ProgrammingResult,
					Details:           rep.Details,
					ServerError:       "operation received RIB_PROGRAMMED but not FIB_PROGRAMMED",
				}
			}

			switch cat {
			case "nh":
				totalNHFailures++
				if len(nhFailures) < 10 {
					nhFailures = append(nhFailures, failRes)
				}
			case "nhg":
				totalNHGFailures++
				if len(nhgFailures) < 10 {
					nhgFailures = append(nhgFailures, failRes)
				}
			case "ipv4":
				totalIPv4Failures++
				if len(ipv4Failures) < 10 {
					ipv4Failures = append(ipv4Failures, failRes)
				}
			case "ipv6":
				totalIPv6Failures++
				if len(ipv6Failures) < 10 {
					ipv6Failures = append(ipv6Failures, failRes)
				}
			default:
				totalOtherFailures++
				if len(otherFailures) < 10 {
					otherFailures = append(otherFailures, failRes)
				}
			}
		}
	}

	for _, res := range otherResults {
		if res.ServerError != "" || res.ClientError != "" {
			totalOtherFailures++
			if len(otherFailures) < 10 {
				otherFailures = append(otherFailures, res)
			}
		}
	}

	if otherTotalCount > 0 {
		t.Logf("ValidateGRIBIResults: %d total operations results (%d NHs, %d NHGs, %d IPv4s, %d IPv6s, %d Other)", len(opOrder)+len(otherResults), nhTotalCount, nhgTotalCount, ipv4TotalCount, ipv6TotalCount, otherTotalCount)
	} else {
		t.Logf("ValidateGRIBIResults: %d total operations results (%d NHs, %d NHGs, %d IPv4s, %d IPv6s)", len(opOrder), nhTotalCount, nhgTotalCount, ipv4TotalCount, ipv6TotalCount)
	}

	hasFailure := false

	if len(nhFailures) > 0 {
		t.Errorf("First %d Next Hop failures (Total: %d/%d):", len(nhFailures), totalNHFailures, nhTotalCount)
		for index, op := range nhFailures {
			t.Errorf("  [%d] %v", index, op)
		}
		hasFailure = true
	} else if nhTotalCount > 0 {
		t.Logf("All Next Hop operations succeeded")
	}

	if len(nhgFailures) > 0 {
		t.Errorf("First %d Next Hop Group failures (Total: %d/%d):", len(nhgFailures), totalNHGFailures, nhgTotalCount)
		for index, op := range nhgFailures {
			t.Errorf("  [%d] %v", index, op)
		}
		hasFailure = true
	} else if nhgTotalCount > 0 {
		t.Logf("All Next Hop Group operations succeeded")
	}

	if len(ipv4Failures) > 0 {
		t.Errorf("First %d IPv4 Entry failures (Total: %d/%d):", len(ipv4Failures), totalIPv4Failures, ipv4TotalCount)
		for index, op := range ipv4Failures {
			t.Errorf("  [%d] %v", index, op)
		}
		hasFailure = true
	} else if ipv4TotalCount > 0 {
		t.Logf("All IPv4 Entry operations succeeded")
	}

	if len(ipv6Failures) > 0 {
		t.Errorf("First %d IPv6 Entry failures (Total: %d/%d):", len(ipv6Failures), totalIPv6Failures, ipv6TotalCount)
		for index, op := range ipv6Failures {
			t.Errorf("  [%d] %v", index, op)
		}
		hasFailure = true
	} else if ipv6TotalCount > 0 {
		t.Logf("All IPv6 Entry operations succeeded")
	}

	if len(otherFailures) > 0 {
		t.Errorf("First %d Other Entry failures (Total: %d/%d):", len(otherFailures), totalOtherFailures, otherTotalCount)
		for index, op := range otherFailures {
			t.Errorf("  [%d] %v", index, op)
		}
		hasFailure = true
	} else if otherTotalCount > 0 {
		t.Logf("All %d Other operations succeeded", otherTotalCount)
	}

	var otherSample []*client.OpResult
	for _, r := range otherResults {
		if len(otherSample) < 5 {
			otherSample = append(otherSample, r)
		}
	}
	if len(otherSample) > 0 {
		t.Logf("First %d Other operations sample:", len(otherSample))
		for index, op := range otherSample {
			t.Logf("  [%d] %v", index, op)
		}
	}

	return hasFailure
}

// VerifyHierarchicalResolution spot-checks TE_VRF_111 prefixes for FIB_PROGRAMMED and non-zero NHG via gNMI AFT.
func VerifyHierarchicalResolution(t *testing.T, c *gribi.Client, dut *ondatra.DUTDevice, samplePrefixes map[string][]string) {
	t.Helper()
	res := c.Fluent(t).Results(t)
	for vrf, prefixes := range samplePrefixes {
		wants := make([]*client.OpResult, 0, len(prefixes))
		for _, pfx := range prefixes {
			wants = append(wants, fluent.OperationResult().WithIPv4Operation(pfx).WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB).AsResult())
		}
		chk.HasResultsCache(t, res, wants, chk.IgnoreOperationID())
		t.Logf("VRF %s: %d prefixes confirmed FIB_PROGRAMMED", vrf, len(prefixes))
		for _, pfx := range prefixes {
			nhg := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(pfx).State()).GetNextHopGroup()
			if nhg == 0 {
				t.Errorf("TE_VRF_111 %s: NHG=0 after FIB_PROGRAMMED (hierarchical resolution failed)", pfx)
			} else {
				t.Logf("TE_VRF_111 %s → NHG %d (OK)", pfx, nhg)
			}
		}
	}
}

// ExpandDecapPrefixes returns the host-address list expanded from DECAP_TE_VRF prefixes.
func ExpandDecapPrefixes(params ScaleParams) []string {
	ips := make([]string, params.NumDecapEntries)
	for i := 0; i < params.NumDecapEntries; i++ {
		ips[i] = fmt.Sprintf("203.%d.%d.1", i/4, (i%4)*64)
	}
	return ips
}

// NewPortFlow creates a gosnappi Flow with the specified fixed packet size and PPS rate, sourced from port1 and destined to port2.
func NewPortFlow(top gosnappi.Config, name string, pktSize uint32, perFlowPPS uint64) gosnappi.Flow {
	f := &otgconfighelpers.Flow{
		FlowName:   name,
		IsTxRxPort: true,
		TxPort:     "port1",
		RxPorts:    []string{"port2"},
		FrameSize:  pktSize,
		PpsRate:    perFlowPPS,
	}
	f.CreateFlow(top)
	// Retrieve the registered gosnappi.Flow, set continuous duration, and return.
	// CreateFlow does not set Duration when PacketsToSend == 0.
	for _, gf := range top.Flows().Items() {
		if gf.Name() == name {
			gf.Duration().Continuous()
			return gf
		}
	}
	// Fallback — should never happen.
	return gosnappi.NewFlow().SetName(name)
}

// NewIMIXPortFlow creates a gosnappi Flow with an IMIX profile and the specified PPS rate, sourced from port1 and destined to port2.
func NewIMIXPortFlow(top gosnappi.Config, name string, perGroupPPS uint64) gosnappi.Flow {
	profile := []otgconfighelpers.SizeWeightPair{
		{Size: 3000, Weight: 7},
		{Size: 1500, Weight: 4},
		{Size: 500, Weight: 1},
	}
	f := &otgconfighelpers.Flow{
		FlowName:          name,
		IsTxRxPort:        true,
		TxPort:            "port1",
		RxPorts:           []string{"port2"},
		PpsRate:           perGroupPPS,
		SizeWeightProfile: &profile,
	}
	f.CreateFlow(top)
	for _, gf := range top.Flows().Items() {
		if gf.Name() == name {
			gf.Duration().Continuous()
			return gf
		}
	}
	return gosnappi.NewFlow().SetName(name)
}

// CountBaseFlows returns the total number of OTG flow groups pushed to the port.
// Both DSCP values per VRF are now expressed inside a single flow via SetValues to reduce flow count.
//
//	encap:    16 VRFs × 2 proto (IPv4+IPv6)          =  32
//	decap:    16 VRFs (or 1 if compacted)            =  16 (or 1)
//	reencap:  16 VRFs × 2 src IPs × 2 proto          =  64
//	transit:  16 VRFs (or 1 if compacted)            =  16 (or 1)
//	repaired: 16 VRFs (or 1 if compacted)            =  16 (or 1)
//	total                                            = 144 (or 99)
//
// Fixed-size: 144 flow groups  ✓ (< 256 limit)
// IMIX:       144 flow groups  ✓ (SetChoice(WEIGHT) = 1 group per VRF)
func CountBaseFlows(compactOTGFlows bool, testRepair bool, params ScaleParams) int {
	transitFlows := params.NumEncapVRFs
	if compactOTGFlows {
		transitFlows = 1
	}
	if testRepair {
		return transitFlows
	}
	decapFlows := params.NumEncapVRFs
	repairedFlows := params.NumEncapVRFs
	if compactOTGFlows {
		decapFlows = 1
		repairedFlows = 1
	}
	return params.NumEncapVRFs*2 + // encap IPv4+IPv6
		decapFlows + // decap
		params.NumEncapVRFs*len(outerSrcs)*2 + // reencap IPv4+IPv6
		transitFlows + // transit
		repairedFlows // repaired
}

// MakeFlowCreator returns a function that creates either fixed-size or IMIX flows based on the imix bool.
func MakeFlowCreator(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) func(string) gosnappi.Flow {
	if imix {
		return func(name string) gosnappi.Flow {
			return NewIMIXPortFlow(top, name, pps)
		}
	}
	return func(name string) gosnappi.Flow {
		return NewPortFlow(top, name, pktSize, pps)
	}
}

// IPIncrement defines the parameters for an incrementing IP pattern in a flow.
type IPIncrement struct {
	Start string
	Step  string
	Count uint32
}

// setIPv4Dst applies the given destination IP configuration to the flow pattern.
func setIPv4Dst(dst gosnappi.PatternFlowIpv4Dst, val any) {
	switch v := val.(type) {
	case string:
		dst.SetValue(v)
	case []string:
		dst.SetValues(v)
	case IPIncrement:
		dst.Increment().SetStart(v.Start).SetStep(v.Step).SetCount(v.Count)
	default:
		panic(fmt.Sprintf("unsupported IPv4 destination type: %T", val))
	}
}

// createIPv4InIPv4Flow is a helper to reduce duplication when building flows with an IPv4-in-IPv4 header.
func createIPv4InIPv4Flow(newFlow func(string) gosnappi.Flow, name, dutIngressMAC, outerSrc string, dscpVals []uint32, outerDst, innerDst any) gosnappi.Flow {
	f := newFlow(name)

	eth := f.Packet().Add().Ethernet()
	eth.Src().SetValue(ATEPort1MAC)
	eth.Dst().SetValue(dutIngressMAC)

	outer := f.Packet().Add().Ipv4()
	outer.Src().SetValue(outerSrc)
	setIPv4Dst(outer.Dst(), outerDst)
	outer.Priority().Dscp().Phb().SetValues(dscpVals)

	inner := f.Packet().Add().Ipv4()
	inner.Src().SetValue(ATEPort1IPv4)
	setIPv4Dst(inner.Dst(), innerDst)
	inner.Priority().Dscp().Phb().SetValues(dscpVals)

	return f
}

// BuildEncapFlows builds fixed-size/imix encap flows for all encap VRFs. IPv4 and IPv6 inners are separate flows since the inner src/dst formats differ.
func BuildEncapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, params ScaleParams) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
	for vi := range encapVRFNames {
		d1, d2 := EncapVRFDSCP(vi)
		dscpVals := []uint32{uint32(d1), uint32(d2)}

		f4 := newFlow(fmt.Sprintf("encap_ipv4_vrf_%d", vi))
		eth4 := f4.Packet().Add().Ethernet()
		eth4.Src().SetValue(ATEPort1MAC)
		eth4.Dst().SetValue(dutIngressMAC)
		ip4 := f4.Packet().Add().Ipv4()
		ip4.Src().SetValue(ATEPort1IPv4)
		ip4.Dst().Increment().SetStart(fmt.Sprintf("200.%d.0.1", vi)).SetStep(CommonPrefixStep).SetCount(uint32(params.NumEncapIPv4PerVRF))
		ip4.Priority().Dscp().Phb().SetValues(dscpVals)
		flows = append(flows, f4)

		f6 := newFlow(fmt.Sprintf("encap_ipv6_vrf_%d", vi))
		eth6 := f6.Packet().Add().Ethernet()
		eth6.Src().SetValue(ATEPort1MAC)
		eth6.Dst().SetValue(dutIngressMAC)
		ip6 := f6.Packet().Add().Ipv6()
		ip6.Src().SetValue(EncapIPv6InnerSrc)
		ip6.Dst().Increment().SetStart(fmt.Sprintf("2001:db8:%x::1", vi)).SetStep(CommonIPv6PrefixStep).SetCount(uint32(params.NumEncapIPv6PerVRF))

		ip6.TrafficClass().SetValues([]uint32{
			uint32(d1) << 2,
			uint32(d2) << 2,
		})
		flows = append(flows, f6)
	}
	return flows
}

// BuildDecapFlows builds fixed-size/imix decap flows for all encap VRFs. Both DSCPs per VRF are expressed via SetValues in a single flow since the outer header is the same.
func BuildDecapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, compact bool, params ScaleParams) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	outerDecapDsts := ExpandDecapPrefixes(params)
	atePort2Ips, _ := iputil.GenerateIPsWithStep(ATEPort2IPv4Start, params.NumPort2VLANs, PortIPv4Step)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	createFlow := func(name string, dscpVals []uint32, innerDstIPs []string) {
		f := createIPv4InIPv4Flow(newFlow, name, dutIngressMAC, IPv4OuterSrc111, dscpVals, outerDecapDsts, innerDstIPs)
		flows = append(flows, f)
	}

	if compact {
		createFlow("decap_vrf_all_src_111", getAllEncapDSCPVals(params.NumEncapVRFs), atePort2Ips)
	} else {
		subsetCount := max(1, params.NumPort2VLANs*params.DecapDestsSubsetPct/100)
		encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
		for vi := range encapVRFNames {
			d1, d2 := EncapVRFDSCP(vi)
			var innerDstIPs []string
			for j := 0; j < subsetCount; j++ {
				idx := (vi*subsetCount + j) % params.NumPort2VLANs
				innerDstIPs = append(innerDstIPs, atePort2Ips[idx])
			}
			createFlow(fmt.Sprintf("decap_vrf_%d_src_111", vi), []uint32{uint32(d1), uint32(d2)}, innerDstIPs)
		}
	}

	return flows
}

// BuildReencapFlows builds fixed-size/imix reencap flows for all encap VRFs.
func BuildReencapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, params ScaleParams) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	decapDsts := ExpandDecapPrefixes(params)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
	for vi := range encapVRFNames {
		d1, d2 := EncapVRFDSCP(vi)
		dscpVals := []uint32{uint32(d1), uint32(d2)}

		for _, outerSrc := range outerSrcs {
			tag := outerSrc[len(outerSrc)-3:]

			// ---------- IPv4 inner ----------
			f4 := newFlow(fmt.Sprintf("reencap_ipv4_vrf_%d_src_%s", vi, tag))
			eth4 := f4.Packet().Add().Ethernet()
			eth4.Src().SetValue(ATEPort1MAC)
			eth4.Dst().SetValue(dutIngressMAC)

			o4 := f4.Packet().Add().Ipv4()
			o4.Src().SetValue(outerSrc)
			o4.Dst().SetValues(decapDsts)
			o4.Priority().Dscp().Phb().SetValues(dscpVals)

			i4 := f4.Packet().Add().Ipv4()
			i4.Src().SetValue(ATEPort1IPv4)
			i4.Dst().Increment().SetStart(fmt.Sprintf("200.%d.0.1", vi)).SetStep(CommonPrefixStep).SetCount(uint32(params.NumEncapIPv4PerVRF))
			i4.Priority().Dscp().Phb().SetValues(dscpVals)

			flows = append(flows, f4)

			// ---------- IPv6 inner ----------
			f6 := newFlow(fmt.Sprintf("reencap_ipv6_vrf_%d_src_%s", vi, tag))
			eth6 := f6.Packet().Add().Ethernet()
			eth6.Src().SetValue(ATEPort1MAC)
			eth6.Dst().SetValue(dutIngressMAC)

			o6 := f6.Packet().Add().Ipv4()
			o6.Src().SetValue(outerSrc)
			o6.Dst().SetValues(decapDsts)
			o6.Priority().Dscp().Phb().SetValues(dscpVals)

			i6 := f6.Packet().Add().Ipv6()
			i6.Src().SetValue(EncapIPv6InnerSrc)
			i6.Dst().Increment().SetStart(fmt.Sprintf("2001:db8:%x::1", vi)).SetStep(CommonIPv6PrefixStep).SetCount(uint32(params.NumEncapIPv6PerVRF))
			i6.TrafficClass().SetValues([]uint32{
				uint32(d1) << 2,
				uint32(d2) << 2,
			})

			flows = append(flows, f6)
		}
	}
	return flows
}

// BuildTransitFlows builds fixed-size/imix transit flows for all encap VRFs.
func BuildTransitFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, compact bool, params ScaleParams) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	createFlow := func(name string, dscpVals []uint32) {
		f := createIPv4InIPv4Flow(newFlow, name, dutIngressMAC, IPv4OuterSrc111, dscpVals, IPIncrement{
			Start: TransitVRF111PrefixStart,
			Step:  CommonPrefixStep,
			Count: uint32(params.NumTransitIPv4),
		}, TransitIPv4InnerDst)

		flows = append(flows, f)
	}

	if compact {
		createFlow("transit_encap_te_vrf_all", getAllEncapDSCPVals(params.NumEncapVRFs))
	} else {
		encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
		for vi := range encapVRFNames {
			d1, d2 := EncapVRFDSCP(vi)
			createFlow(fmt.Sprintf("transit_encap_te_vrf_%d", vi), []uint32{uint32(d1), uint32(d2)})
		}
	}

	return flows
}

// BuildRepairedFlows builds fixed-size/imix flows for all repaired VRFs.
func BuildRepairedFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, compact bool, params ScaleParams) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	createFlow := func(name string, dscpVals []uint32) {
		f := createIPv4InIPv4Flow(newFlow, name, dutIngressMAC, IPv4OuterSrc222, dscpVals, IPIncrement{
			Start: TransitVRF222PrefixStart,
			Step:  CommonPrefixStep,
			Count: uint32(params.NumTransitIPv4),
		}, RepairIPv4InnerDst)

		flows = append(flows, f)
	}

	if compact {
		createFlow("repaired_encap_te_vrf_all", getAllEncapDSCPVals(params.NumEncapVRFs))
	} else {
		encapVRFNames := BuildEncapVRFNames(params.NumEncapVRFs)
		for vi := range encapVRFNames {
			d1, d2 := EncapVRFDSCP(vi)
			createFlow(fmt.Sprintf("repaired_encap_te_vrf_%d", vi), []uint32{uint32(d1), uint32(d2)})
		}
	}

	return flows
}

// GetDUTMACAddress retrieves the MAC address for the given interface and neighbor IP.
func GetDUTMACAddress(t *testing.T, ate *ondatra.ATEDevice, intfName string, neighborIP string) string {
	t.Helper()
	t.Logf("Fetching MAC address for %s neighbor %s", intfName, neighborIP)
	llAddress, found := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Interface(intfName).Ipv4Neighbor(neighborIP).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not get the LinkLayerAddress for %s neighbor %s", intfName, neighborIP)
	}
	dutIngressMAC, _ := llAddress.Val()
	t.Logf("Resolved MAC address: %s", dutIngressMAC)
	return dutIngressMAC
}

// RunEndToEndTrafficValidation executes the end-to-end traffic validation for all scenarios. It registers flows, configures capture, runs traffic, and validates via otgvalidationhelpers and packetvalidationhelpers.
func RunEndToEndTrafficValidation(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, dutIngressMAC string, imix bool, testRepair bool, enablePacketCapture bool, compactOTGFlows bool, params ScaleParams) {
	t.Helper()
	baseFlows := max(CountBaseFlows(compactOTGFlows, testRepair, params), 1)
	perFlowPPS := params.TrafficRateMpps / uint64(baseFlows)
	if perFlowPPS == 0 {
		perFlowPPS = 1
	}
	t.Logf("Traffic : imix=%v, %d base flow groups, %d pps/group -> ~%d Mpps aggregate", imix, baseFlows, perFlowPPS, perFlowPPS*uint64(baseFlows)/1_000_000)

	decapPfxSet := ExpandDecapPrefixes(params)
	expectations := map[string]FlowExpectation{}

	// addFlows registers each flow's expectation. The VRF index is extracted
	// from the flow's position in the slice so both DSCP values for that VRF
	// can be stored in ExpectedDSCPs — either is a valid egress DSCP.
	addFlows := func(builtFlows []gosnappi.Flow, sc TrafficScenario, outerSrc string, wantEncap bool, flowsPerVRF int) {
		isCompacted := compactOTGFlows && (sc == ScenarioDecap || sc == ScenarioTransit || sc == ScenarioRepaired)
		for fi, f := range builtFlows {
			var expectedDSCPs []uint32
			if isCompacted {
				expectedDSCPs = getAllEncapDSCPVals(params.NumEncapVRFs)
			} else {
				vi := fi / flowsPerVRF // VRF index from position in scenario slice
				d1, d2 := EncapVRFDSCP(vi)
				expectedDSCPs = []uint32{uint32(d1), uint32(d2)}
			}
			expOuterSrc := outerSrc
			if testRepair && sc == ScenarioTransit {
				expOuterSrc = IPv4OuterSrc222
			}
			expectations[f.Name()] = FlowExpectation{
				Scenario:         sc,
				ExpectedOuterSrc: expOuterSrc,
				ExpectedDSCPs:    expectedDSCPs,
				WantEncapPresent: wantEncap,
				DecapPrefixSet:   decapPfxSet,
			}
		}
	}

	// Build all flows directly into top so NewPortFlow / NewIMIXPortFlow can register them in the gosnappi config and return references.
	top.Flows().Clear()
	allFlows := make([]gosnappi.Flow, 0)

	pktSize := uint32(64)
	if imix {
		pktSize = 0
	}

	builders := []struct {
		build       func(gosnappi.Config, uint32, uint64, bool, string, ScaleParams) []gosnappi.Flow
		scenario    TrafficScenario
		outerSrc    string
		needCapture bool
		multiplier  int
	}{
		{BuildEncapFlows, ScenarioEncap, IPv4OuterSrc111, true, 2},
		{func(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, params ScaleParams) []gosnappi.Flow {
			return BuildDecapFlows(top, pktSize, pps, imix, dutIngressMAC, compactOTGFlows, params)
		}, ScenarioDecap, "", false, 1},
		{BuildReencapFlows, ScenarioReencap, "", true, 4},
		{func(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, params ScaleParams) []gosnappi.Flow {
			return BuildTransitFlows(top, pktSize, pps, imix, dutIngressMAC, compactOTGFlows, params)
		}, ScenarioTransit, IPv4OuterSrc111, true, 1},
		{func(top gosnappi.Config, pktSize uint32, pps uint64, imix bool, dutIngressMAC string, params ScaleParams) []gosnappi.Flow {
			return BuildRepairedFlows(top, pktSize, pps, imix, dutIngressMAC, compactOTGFlows, params)
		}, ScenarioRepaired, IPv4OuterSrc222, true, 1},
	}

	for _, b := range builders {
		if testRepair && b.scenario != ScenarioTransit {
			continue
		}
		flows := b.build(top, pktSize, perFlowPPS, imix, dutIngressMAC, params)
		addFlows(flows, b.scenario, b.outerSrc, b.needCapture, b.multiplier)
		allFlows = append(allFlows, flows...)
	}

	var capVal *packetvalidationhelpers.PacketValidation
	if enablePacketCapture {
		// Clear capture
		packetvalidationhelpers.ClearCapture(t, top, ate)

		// Configure capture on port2 via packetvalidationhelpers and push config.
		capVal = &packetvalidationhelpers.PacketValidation{
			PortName:    "port2",
			CaptureName: "cap_port2",
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, capVal)
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Start capture (if enabled), run traffic, stop capture.
	var cs gosnappi.ControlState
	if enablePacketCapture {
		// StartCapture returns the ControlState it armed; StopCapture reuses it to issue the STOP command on the same port-capture object.
		cs = packetvalidationhelpers.StartCapture(t, ate)
	}

	if testRepair {
		dp2 := dut.Port(t, "port2")
		// Disable subinterfaces based on ShutdownVLANPct.
		downCount := max(params.NumPort2VLANs*ShutdownVLANPct/100, 1)

		setDUTSubinterfaceAdminState(t, dut, dp2.Name(), 1, downCount, false)
		defer func() {
			setDUTSubinterfaceAdminState(t, dut, dp2.Name(), 1, downCount, true)
			verifySubinterfaceStatus(t, dut, dp2.Name(), 1, downCount, true)
		}()
		verifySubinterfaceStatus(t, dut, dp2.Name(), 1, downCount, false)
	}

	ate.OTG().StartTraffic(t)
	time.Sleep(params.TrafficDuration)
	ate.OTG().StopTraffic(t)

	if enablePacketCapture {
		packetvalidationhelpers.StopCapture(t, ate, cs)
	}

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)

	// Step 1: Zero-loss check via otgvalidationhelpers per flow.
	for _, f := range allFlows {
		v := &otgvalidationhelpers.OTGValidation{
			Flow: &otgvalidationhelpers.FlowParams{
				Name:         f.Name(),
				TolerancePct: float32(params.TrafficLossTol),
			},
		}
		if err := v.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("Zero-loss check: %v", err)
		}
	}

	// Step 2: Deep packet inspection.
	// Build per-scenario PacketValidation descriptors and delegate to
	// packetvalidationhelpers.CaptureAndValidatePackets which uses gopacket.
	if enablePacketCapture {
		ValidateCapturedPackets(t, ate, capVal, expectations)
	}
}

// ValidateCapturedPackets performs deep packet inspection on the captured packets using gopacket, validating against the expectations for each flow and scenario.
func ValidateCapturedPackets(t *testing.T, ate *ondatra.ATEDevice, capVal *packetvalidationhelpers.PacketValidation, expectations map[string]FlowExpectation) {
	t.Helper()

	// Group expectations by scenario — one validation call per scenario is
	// sufficient because all flows within a scenario share the same header shape.
	type scenarioGroup struct {
		exp      FlowExpectation
		flowName string // representative flow name for logging
	}
	seen := map[TrafficScenario]scenarioGroup{}
	for name, exp := range expectations {
		if _, ok := seen[exp.Scenario]; !ok {
			seen[exp.Scenario] = scenarioGroup{exp: exp, flowName: name}
		}
	}

	for scenario, grp := range seen {
		exp := grp.exp
		t.Logf("Validating scenario %s (representative flow: %s)", scenario, grp.flowName)

		// Derive the TOS byte from the first expected DSCP value.
		// OTG cycles through SetValues across packets, so either DSCP in the set
		// is valid; we spot-check the first one as a representative sample.
		tosByte := uint8(0)
		if len(exp.ExpectedDSCPs) > 0 {
			tosByte = uint8(exp.ExpectedDSCPs[0] << 2)
		}

		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    capVal.PortName,
			CaptureName: capVal.CaptureName,
			IPv4Layer: &packetvalidationhelpers.IPv4Layer{
				// DstIP is repurposed here to carry the expected outer-src IP that
				// the DUT stamps on encapped/transit/repaired egress packets.
				// For Decap (ExpectedOuterSrc == "") this field is left empty and
				// the DstIP check inside validateIPv4Header will be skipped because
				// the captured decapped packet's dst is the inner-packet's original dst.
				DstIP: exp.ExpectedOuterSrc,
				Tos:   tosByte,
			},
		}

		switch scenario {
		case ScenarioEncap, ScenarioTransit, ScenarioRepaired:
			// Outer packet must be IP-in-IP (protocol 4); inner IPv4 must parse.
			// packetvalidationhelpers.ValidateInnerIPv4Header looks for a GRE layer,
			// which does not apply here. We therefore validate only the outer header
			// via ValidateIPv4Header (which checks Protocol, DstIP, TOS) and rely on
			// the protocol=4 check to confirm encapsulation is present.
			pv.IPv4Layer.Protocol = 4
			pv.Validations = []packetvalidationhelpers.ValidationType{
				packetvalidationhelpers.ValidateIPv4Header,
			}

		case ScenarioDecap:
			// After decap the egress packet has no outer tunnel header.
			// Suppress the Protocol check (we do not know the inner protocol a priori)
			// and skip DstIP (the decapped dst is the original inner-packet dst, not
			// a DUT-stamped tunnel address).
			pv.IPv4Layer.SkipProtocolCheck = true
			pv.IPv4Layer.DstIP = "" // do not constrain dst for decap
			pv.Validations = []packetvalidationhelpers.ValidationType{
				packetvalidationhelpers.ValidateIPv4Header,
			}

		case ScenarioReencap:
			// IP-in-IP must be present (protocol=4).
			// The outer-dst must NOT be in the original decap prefix set —
			// a non-empty DstIP in IPv4Layer would constrain to an exact address,
			// which is too strict for reencap (the new outer-dst is from the encap
			// NHG, not from the decap set).  We therefore skip DstIP here and rely
			// on the protocol=4 check to confirm re-encapsulation happened.
			pv.IPv4Layer.Protocol = 4
			pv.IPv4Layer.DstIP = "" // new outer-dst comes from encap NHG; don't pin
			pv.Validations = []packetvalidationhelpers.ValidationType{
				packetvalidationhelpers.ValidateIPv4Header,
			}
		}

		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			t.Errorf("Scenario %s: CaptureAndValidatePackets: %v", scenario, err)
		}
	}
}

// RebootChassis reboots the DUT and verifies it comes back up with the same software version and all components responsive.
func RebootChassis(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   RebootDelay * OneSecondInNanoSecond,
		Message: "Reboot chassis with delay",
		Force:   true,
	}

	versions := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	expectedVersion := FetchUniqueItems(t, versions)
	sort.Strings(expectedVersion)
	t.Logf("DUT software version: %v", expectedVersion)

	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	t.Logf("DUT components status pre reboot: %v", preRebootCompStatus)

	preRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	preCompMatrix := []string{}
	for _, preComp := range preRebootCompDebug {
		if preComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
			preCompMatrix = append(preCompMatrix, preComp.GetName()+":"+preComp.GetOperStatus().String())
		}
	}

	t.Logf("Starting reboot")
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	prevTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
	if err != nil {
		t.Fatalf("Failed parsing current-datetime: %s", err)
	}
	start := time.Now()

	t.Logf("Send reboot request: %v", rebootRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	t.Logf("Validating DUT remains reachable for at least %d seconds", RebootDelay)
	for {
		time.Sleep(10 * time.Second)
		t.Logf("Time elapsed %.2f seconds since reboot was requested.", time.Since(start).Seconds())
		if time.Since(start).Seconds() > RebootDelay {
			t.Logf("Time elapsed %.2f seconds > %d reboot delay", time.Since(start).Seconds(), RebootDelay)
			break
		}
		timeVal := gnmi.Lookup(t, dut, gnmi.OC().System().CurrentDatetime().State())
		if !timeVal.IsPresent() {
			t.Logf("Device became unreachable at %.2f seconds (expected delay: %d). Breaking reachability loop.", time.Since(start).Seconds(), RebootDelay)
			break
		}
		latestTimeStr, _ := timeVal.Val()
		latestTime, err := time.Parse(time.RFC3339, latestTimeStr)
		if err != nil {
			t.Fatalf("Failed parsing current-datetime: %s", err)
		}
		if latestTime.Before(prevTime) || latestTime.Equal(prevTime) {
			t.Errorf("Get latest system time: got %v, want newer time than %v", latestTime, prevTime)
		}
		prevTime = latestTime
	}

	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Seconds()) > MaxRebootTime {
			t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), MaxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f seconds", time.Since(startReboot).Seconds())

	bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
	if bootTimeAfterReboot <= bootTimeBeforeReboot {
		t.Errorf("Get boot time: got %v, want > %v", bootTimeAfterReboot, bootTimeBeforeReboot)
	}

	startComp := time.Now()
	t.Logf("Wait for all the components on DUT to come up")

	for {
		postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
		postRebootCompDebug := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
		postCompMatrix := []string{}
		for _, postComp := range postRebootCompDebug {
			if postComp.GetOperStatus() != oc.PlatformTypes_COMPONENT_OPER_STATUS_UNSET {
				postCompMatrix = append(postCompMatrix, postComp.GetName()+":"+postComp.GetOperStatus().String())
			}
		}

		if len(preRebootCompStatus) == len(postRebootCompStatus) {
			t.Logf("All components on the DUT are in responsive state")
			time.Sleep(10 * time.Second)
			break
		}

		if uint64(time.Since(startComp).Seconds()) > MaxCompWaitTime {
			t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
			if rebootDiff := cmp.Diff(preCompMatrix, postCompMatrix); rebootDiff != "" {
				t.Logf("[DEBUG] Unexpected diff after reboot (-component missing from pre reboot, +component added from pre reboot): %v ", rebootDiff)
			}
			t.Fatalf("There's a difference in components obtained in pre reboot: %v and post reboot: %v.", len(preRebootCompStatus), len(postRebootCompStatus))
		}
		time.Sleep(10 * time.Second)
	}

	versions = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	swVersion := FetchUniqueItems(t, versions)
	sort.Strings(swVersion)
	t.Logf("DUT software version after reboot: %v", swVersion)
	if diff := cmp.Diff(expectedVersion, swVersion); diff != "" {
		t.Errorf("Software version differed (-want +got):\n%v", diff)
	}
}

// FetchUniqueItems returns a slice of unique strings from the input slice.
func FetchUniqueItems(t *testing.T, s []string) []string {
	t.Helper()
	itemExisted := make(map[string]bool)
	var uniqueList []string
	for _, item := range s {
		if _, ok := itemExisted[item]; !ok {
			itemExisted[item] = true
			uniqueList = append(uniqueList, item)
		} else {
			t.Logf("Detected duplicated item: %v", item)
		}
	}
	return uniqueList
}

func formatNHGLoadBalancing(params []NHGLoadBalancingParams) string {
	if len(params) == 0 {
		return "[]"
	}
	var items []string
	for _, p := range params {
		items = append(items, fmt.Sprintf("{%d%% -> %dNH}", p.Pct, p.NumNextHops))
	}
	return fmt.Sprintf("[%s]", strings.Join(items, ", "))
}

func formatNHGWeight(params []NHGWeightParams) string {
	if len(params) == 0 {
		return "[]"
	}
	var items []string
	for _, p := range params {
		cfgStr := "ECMP"
		if w, ok := p.Config.(WCMP); ok {
			cfgStr = fmt.Sprintf("WCMP1in%d", w.WeightGranularity)
		} else if ew, ok := p.Config.(ExplicitWeights); ok {
			cfgStr = fmt.Sprintf("Explicit%v", ew.Weights)
		}
		items = append(items, fmt.Sprintf("{%d%% -> %s}", p.Pct, cfgStr))
	}
	return fmt.Sprintf("[%s]", strings.Join(items, ", "))
}

// LogScaleParams logs all scale configuration parameters for the test run.
func LogScaleParams(t *testing.T, params ScaleParams) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("\n============================================================\n")
	sb.WriteString("                 gRIBI Scale Parameters                     \n")
	sb.WriteString("============================================================\n")
	sb.WriteString(fmt.Sprintf("  GRIBIBatchSize        : %d\n", params.GRIBIBatchSize))
	sb.WriteString(fmt.Sprintf("  GRIBIBatchTimeout     : %v\n", params.GRIBIBatchTimeout))
	sb.WriteString("  [Default VRF]\n")
	sb.WriteString(fmt.Sprintf("    NumDefaultNH        : %d\n", params.NumDefaultNH))
	sb.WriteString(fmt.Sprintf("    NumDefaultNHG       : %d\n", params.NumDefaultNHG))
	sb.WriteString(fmt.Sprintf("    NumDefaultIPv4      : %d\n", params.NumDefaultIPv4))
	sb.WriteString(fmt.Sprintf("    DefaultNHGLoadBal   : %s\n", formatNHGLoadBalancing(params.DefaultNHGLoadBalance)))
	sb.WriteString(fmt.Sprintf("    DefaultNHGWeight    : %s\n", formatNHGWeight(params.DefaultNHGWeight)))
	sb.WriteString("  [Transit VRF]\n")
	sb.WriteString(fmt.Sprintf("    NumTransitNH        : %d\n", params.NumTransitNH))
	sb.WriteString(fmt.Sprintf("    NumTransitNHG       : %d\n", params.NumTransitNHG))
	sb.WriteString(fmt.Sprintf("    NumTransitIPv4      : %d\n", params.NumTransitIPv4))
	sb.WriteString(fmt.Sprintf("    TransitNHGLoadBal   : %s\n", formatNHGLoadBalancing(params.TransitNHGLoadBalance)))
	sb.WriteString(fmt.Sprintf("    TransitNHGWeight    : %s\n", formatNHGWeight(params.TransitNHGWeight)))
	sb.WriteString("  [Repair VRF]\n")
	sb.WriteString(fmt.Sprintf("    NumRepairNHG        : %d\n", params.NumRepairNHG))
	sb.WriteString(fmt.Sprintf("    NumRepairIPv4       : %d\n", params.NumRepairIPv4))
	sb.WriteString("  [Encap VRF]\n")
	sb.WriteString(fmt.Sprintf("    NumEncapVRFs        : %d\n", params.NumEncapVRFs))
	sb.WriteString(fmt.Sprintf("    NumEncapNHPerVRF    : %d\n", params.NumEncapNHPerVRF))
	sb.WriteString(fmt.Sprintf("    NumEncapNHGPerVRF   : %d\n", params.NumEncapNHGPerVRF))
	sb.WriteString(fmt.Sprintf("    NumEncapIPv4PerVRF  : %d\n", params.NumEncapIPv4PerVRF))
	sb.WriteString(fmt.Sprintf("    NumEncapIPv6PerVRF  : %d\n", params.NumEncapIPv6PerVRF))
	sb.WriteString(fmt.Sprintf("    EncapNHGLoadBal     : %s\n", formatNHGLoadBalancing(params.EncapNHGLoadBalance)))
	sb.WriteString(fmt.Sprintf("    EncapNHGWeight      : %s\n", formatNHGWeight(params.EncapNHGWeight)))
	sb.WriteString("  [Decap VRF]\n")
	sb.WriteString(fmt.Sprintf("    NumDecapEntries     : %d\n", params.NumDecapEntries))
	sb.WriteString(fmt.Sprintf("    DecapDestsSubsetPct : %d%%\n", params.DecapDestsSubsetPct))
	sb.WriteString("  [Port / Traffic]\n")
	sb.WriteString(fmt.Sprintf("    NumPort2VLANs       : %d\n", params.NumPort2VLANs))
	sb.WriteString(fmt.Sprintf("    TrafficRateMpps     : %d\n", params.TrafficRateMpps))
	sb.WriteString(fmt.Sprintf("    TrafficDuration     : %v\n", params.TrafficDuration))
	sb.WriteString(fmt.Sprintf("    TrafficLossTol      : %d\n", params.TrafficLossTol))
	sb.WriteString("============================================================\n")
	t.Log(sb.String())
}

// validateDefaultVRFParams verifies scale parameters specific to the Default VRF.
func validateDefaultVRFParams(t *testing.T, params ScaleParams) {
	t.Helper()
	if params.NumDefaultNH <= 0 {
		t.Fatalf("validateDefaultVRFParams: NumDefaultNH (%d) must be greater than 0", params.NumDefaultNH)
	}
	if params.NumDefaultNHG <= 0 {
		t.Fatalf("validateDefaultVRFParams: NumDefaultNHG (%d) must be greater than 0", params.NumDefaultNHG)
	}
	validateNHGLoadBalance(t, "DefaultNHGLoadBalance", params.DefaultNHGLoadBalance)
	validateNHGWeight(t, "DefaultNHGWeight", params.DefaultNHGWeight)
	numNHPart, numNHGPart := splitDefaultVRFPrimaryBackup(params)
	if !AreAllNHsReferenced(numNHPart, numNHGPart, params.DefaultNHGLoadBalance) {
		t.Fatalf("validateDefaultVRFParams: not all Default VRF next hops (%d) will be referenced by next hop groups (%d) given DefaultNHGLoadBalance", params.NumDefaultNH, params.NumDefaultNHG)
	}
}

// validateEncapVRFParams verifies scale parameters specific to the Encap VRFs.
func validateEncapVRFParams(t *testing.T, params ScaleParams) {
	t.Helper()
	if params.NumEncapVRFs <= 0 {
		t.Fatalf("validateEncapVRFParams: NumEncapVRFs (%d) must be greater than 0", params.NumEncapVRFs)
	}
	if params.NumEncapNHPerVRF <= 0 {
		t.Fatalf("validateEncapVRFParams: NumEncapNHPerVRF (%d) must be greater than 0", params.NumEncapNHPerVRF)
	}
	if params.NumEncapNHGPerVRF <= 0 {
		t.Fatalf("validateEncapVRFParams: NumEncapNHGPerVRF (%d) must be greater than 0", params.NumEncapNHGPerVRF)
	}
	if params.NumTransitIPv4 == 0 {
		t.Fatalf("validateEncapVRFParams: NumTransitIPv4 must be greater than 0 when NumEncapNHPerVRF (%d) is greater than 0", params.NumEncapNHPerVRF)
	}
	validateNHGLoadBalance(t, "EncapNHGLoadBalance", params.EncapNHGLoadBalance)
	validateNHGWeight(t, "EncapNHGWeight", params.EncapNHGWeight)

	if !AreAllNHsReferenced(params.NumEncapNHPerVRF, params.NumEncapNHGPerVRF, params.EncapNHGLoadBalance) {
		t.Fatalf("validateEncapVRFParams: not all Encap VRF next hops (%d) will be referenced by next hop groups (%d) given EncapNHGLoadBalance", params.NumEncapNHPerVRF, params.NumEncapNHGPerVRF)
	}
}

// validateTransitVRFParams verifies scale parameters specific to the Transit VRFs.
func validateTransitVRFParams(t *testing.T, params ScaleParams) {
	t.Helper()
	if params.NumTransitNH < 2 {
		t.Fatalf("validateTransitVRFParams: NumTransitNH (%d) must be at least 2 to support both primary and repaired transit VRFs", params.NumTransitNH)
	}
	if params.NumTransitNHG < 2 {
		t.Fatalf("validateTransitVRFParams: NumTransitNHG (%d) must be at least 2 to support both primary and repaired transit VRFs", params.NumTransitNHG)
	}
	validateNHGLoadBalance(t, "TransitNHGLoadBalance", params.TransitNHGLoadBalance)
	validateNHGWeight(t, "TransitNHGWeight", params.TransitNHGWeight)

	nhPrimaryCount, nhgPrimaryCount, nhBackupCount, nhgBackupCount := splitTransitVRFPrimaryBackup(params)
	if !AreAllNHsReferenced(nhPrimaryCount, nhgPrimaryCount, params.TransitNHGLoadBalance) {
		t.Fatalf("validateTransitVRFParams: not all primary Transit VRF next hops (%d) will be referenced by next hop groups (%d) given TransitNHGLoadBalance", nhPrimaryCount, nhgPrimaryCount)
	}
	if nhBackupCount > 0 && nhgBackupCount > 0 {
		if !AreAllNHsReferenced(nhBackupCount, nhgBackupCount, params.TransitNHGLoadBalance) {
			t.Fatalf("validateTransitVRFParams: not all backup Transit VRF next hops (%d) will be referenced by next hop groups (%d) given TransitNHGLoadBalance", nhBackupCount, nhgBackupCount)
		}
	}
}

// validateScaleParams verifies that all scale configuration options are logically valid.
func validateScaleParams(t *testing.T, params ScaleParams) {
	t.Helper()

	LogScaleParams(t, params)

	if params.GRIBIBatchTimeout < 0 {
		t.Fatalf("validateScaleParams: GRIBIBatchTimeout (%v) cannot be negative", params.GRIBIBatchTimeout)
	}

	// Default VRF category
	validateDefaultVRFParams(t, params)

	// Encap VRF category
	validateEncapVRFParams(t, params)

	// Decap VRF category
	if params.NumDecapEntries < 0 {
		t.Fatalf("validateScaleParams: NumDecapEntries (%d) cannot be negative", params.NumDecapEntries)
	}
	if params.DecapDestsSubsetPct < 0 || params.DecapDestsSubsetPct > 100 {
		t.Fatalf("validateScaleParams: DecapDestsSubsetPct (%d) must be between 0 and 100", params.DecapDestsSubsetPct)
	}

	// Repair VRF category
	if params.NumRepairIPv4 > 0 && params.NumRepairNHG <= 0 {
		t.Fatalf("validateScaleParams: NumRepairNHG (%d) must be greater than 0 when NumRepairIPv4 (%d) is greater than 0", params.NumRepairNHG, params.NumRepairIPv4)
	}

	// Transit VRF category
	validateTransitVRFParams(t, params)

	// General/Other validations
	if params.NumPort2VLANs <= 0 {
		t.Fatalf("validateScaleParams: NumPort2VLANs (%d) must be greater than 0", params.NumPort2VLANs)
	}
}

func formatUint64Leaf(val *uint64) string {
	if val == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d", *val)
}

func formatUint8Leaf(val *uint8) string {
	if val == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d%%", *val)
}

func formatBoolLeaf(val *bool) string {
	if val == nil {
		return "N/A"
	}
	return fmt.Sprintf("%t", *val)
}

// LogHWUtilization queries hardware resource utilization across all integrated circuit components on the DUT directly without requiring pre-discovered component names.
func LogHWUtilization(t *testing.T, dut *ondatra.DUTDevice, monitorHWUtilization bool, stage string) {
	t.Helper()
	if !monitorHWUtilization {
		return
	}
	resourceVals := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().IntegratedCircuit().Utilization().ResourceAny().State())
	t.Logf("Found %d HW resources for monitoring", len(resourceVals))
	if len(resourceVals) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== [%s] HW Resource Utilization ===\n", stage))
	sb.WriteString(fmt.Sprintf("%-20s | %-35s | %-12s | %-12s | %-12s | %-12s | %-14s | %-19s | %-12s | %-12s | %-10s\n",
		"COMPONENT", "RESOURCE", "USED", "FREE", "MAX LIMIT", "COMMITTED", "HIGH WATERMARK", "LAST HIGH WATERMARK", "THRESH UPPER", "THRESH CLEAR", "EXCEEDED"))
	sb.WriteString(strings.Repeat("-", 195) + "\n")

	entriesLogged := 0
	compSet := make(map[string]bool)
	for _, val := range resourceVals {
		res, ok := val.Val()
		if !ok {
			continue
		}
		compName := "UNKNOWN"
		if path := val.Path; path != nil {
			pathStr := path.String()
			t.Logf("Path: %s", pathStr)
			for _, elem := range path.GetElem() {
				if elem.GetName() == "component" {
					if name, ok := elem.GetKey()["name"]; ok {
						compName = name
						break
					}
				}
			}
		}
		compSet[compName] = true
		entriesLogged++
		sb.WriteString(fmt.Sprintf("%-20s | %-35s | %-12s | %-12s | %-12s | %-12s | %-14s | %-19s | %-12s | %-12s | %-10s\n",
			compName,
			res.GetName(),
			formatUint64Leaf(res.Used),
			formatUint64Leaf(res.Free),
			formatUint64Leaf(res.MaxLimit),
			formatUint64Leaf(res.Committed),
			formatUint64Leaf(res.HighWatermark),
			formatUint64Leaf(res.LastHighWatermark),
			formatUint8Leaf(res.UsedThresholdUpper),
			formatUint8Leaf(res.UsedThresholdUpperClear),
			formatBoolLeaf(res.UsedThresholdUpperExceeded)))
	}

	if entriesLogged > 0 {
		t.Log(sb.String())
		var compList []string
		for comp := range compSet {
			compList = append(compList, comp)
		}
		sort.Strings(compList)
		t.Logf("[%s] Found %d unique components reporting utilization: %v", stage, len(compList), compList)
	}
}

// RunTrafficTestCases iterates through the configured traffic test cases (fixed-size, IMIX, repair)
// and executes end-to-end traffic validation for each.
func RunTrafficTestCases(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, ateConfig gosnappi.Config, dutIngressMAC string, enablePacketCapture, compactOTGFlows bool, params ScaleParams) {
	t.Helper()
	testCases := []TrafficTestCase{
		{Name: "FixedSize_64B", UseIMIX: false, TestRepair: false},
		{Name: "IMIX_Profile", UseIMIX: true, TestRepair: false},
		{Name: "FixedSize_64B_Repair", UseIMIX: false, TestRepair: true},
		{Name: "IMIX_Profile_Repair", UseIMIX: true, TestRepair: true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.UseIMIX {
				if tc.TestRepair {
					t.Logf("Running IMIX traffic — transit scenario only for repair testing, %d Mpps aggregate", params.TrafficRateMpps)
				} else {
					t.Logf("Running IMIX traffic — all 5 scenarios, %d Mpps aggregate", params.TrafficRateMpps)
				}
			} else {
				if tc.TestRepair {
					t.Logf("Running fixed-size (64B) traffic — transit scenario only for repair testing, %d Mpps aggregate", params.TrafficRateMpps)
				} else {
					t.Logf("Running fixed-size (64B) traffic — all 5 scenarios, %d Mpps aggregate", params.TrafficRateMpps)
				}
			}
			RunEndToEndTrafficValidation(t, ate, dut, ateConfig, dutIngressMAC, tc.UseIMIX, tc.TestRepair, enablePacketCapture, compactOTGFlows, params)
		})
	}
}

// RunFullScaleTest runs the complete set of configuration, programming, and traffic tests
// for the given scale parameters.
func RunFullScaleTest(t *testing.T, params ScaleParams, enablePacketCapture, compactOTGFlows, monitorHWUtilization bool) {
	t.Helper()

	validateScaleParams(t, params)

	t.Log("Configuring DUT interfaces, VRFs, and VRF-selection policy")
	dut, defaultVRF := ConfigureDUT(t, params)

	var ate *ondatra.ATEDevice
	var ateConfig gosnappi.Config
	var dutIngressMAC string

	if !*excludeTraffic {
		ate, ateConfig, dutIngressMAC = configureOTG(t, dut, params)
	} else {
		t.Log("Flag -exclude_traffic is set: skipping ATE/IXIA configuration and ARP resolution")
	}

	t.Cleanup(func() {
		FlushGRIBIRoutes(t, dut)
	})

	t.Run("Configure and validate FIB_PROGRAMMED, Hierarchical route structure", func(t *testing.T) {
		ProgramGRIBIRoutes(t, dut, defaultVRF, params, monitorHWUtilization)
	})

	if !*excludeTraffic {
		RunTrafficTestCases(t, ate, dut, ateConfig, dutIngressMAC, enablePacketCapture, compactOTGFlows, params)
	} else {
		t.Log("Flag -exclude_traffic is set: skipping traffic execution sub-tests")
	}
}
