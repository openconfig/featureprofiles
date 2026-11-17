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

// Package plq provides shared helpers and dynamic calculations for gNOI Packet Link Qualification tests.
package plq

import (
	"fmt"
	"math"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
)

const (
	defaultMinGeneratorMTU       = uint64(8184)
	defaultMinGeneratorPPS       = uint64(1e8)
	defaultMinGeneratorBPS       = uint64(1e11)
	defaultTargetPacketSize      = uint32(1500)
	defaultMinEthernetFrame      = uint32(64)
	defaultInjectedPackets       = uint32(10)
	l1FramingOverheadBytes       = uint64(20) // 7B preamble + 1B SFD + 12B IFG
	wireHeadroomPercent          = uint64(95) // Keep 5% headroom below raw physical wire limit.
	defaultQualificationDuration = 60 * time.Second
	defaultLossTolerancePct      = 0.0001 // Maximum allowed TX/RX counter mismatch percentage (0.0001%).
	minDeliveredPacketRatio      = 0.50
	defaultSyncBuffer            = 10 * time.Second
	defaultMinGenSetup           = 30 * time.Second
	defaultMinRefSetup           = 40 * time.Second
	defaultMinRefPostSync        = 30 * time.Second
	defaultMinGenTeardown        = 30 * time.Second
	defaultMinRefTeardown        = 10 * time.Second
	defaultPollGracePeriod       = 120 * time.Second
	// Cisco WBB WAN ports enforce a 180s carrier-delay up timer and require the far-end
	// reflector to wait before enabling PMD loopback (b/529815281, b/568225273).
	ciscoMinSetupDuration = 210 * time.Second
	ciscoRefPreSync       = 30 * time.Second
)

// Config configures packet link qualification test execution.
type Config struct {
	// TargetSpeed filters link discovery to ports of the given Ondatra speed.
	// If zero, all linked DUT port pairs in the testbed are qualified.
	TargetSpeed ondatra.Speed
	// TestDuration overrides the active traffic qualification duration when > 0.
	TestDuration time.Duration
	// RunMaxPPSSubtest controls whether single-link Max-PPS qualification runs.
	RunMaxPPSSubtest bool
	// RunLAGSubtest controls whether single-pair static Member-Link LAG qualification runs.
	RunLAGSubtest bool
}

// LinkEndpoint identifies one side of a qualified physical link.
type LinkEndpoint struct {
	DUT  *ondatra.DUTDevice
	Port *ondatra.Port
}

// LinkPair represents two physically connected DUT ports and their resolved speed.
type LinkPair struct {
	A        LinkEndpoint
	B        LinkEndpoint
	SpeedBPS uint64
}

// CapabilityThresholds holds expected minimum generator capability bounds for a DUT.
type CapabilityThresholds struct {
	MinMTU uint64
	MinPPS uint64
	MinBPS uint64
}

// TrafficParams holds dynamically computed packet rate or injector count, size, and duration parameters.
type TrafficParams struct {
	PacketRate       uint64
	PacketSize       uint32
	TargetBPS        uint64
	Duration         time.Duration
	UseInjector      bool
	PacketCount      uint32
	InjectorLoopback plqpb.PacketInjectorLoopbackMode
}

// Durations holds synchronized generator and reflector RPCSyncedTiming phases.
type Durations struct {
	GenPreSync  time.Duration
	RefPreSync  time.Duration
	GenSetup    time.Duration
	RefSetup    time.Duration
	Test        time.Duration
	GenPostSync time.Duration
	RefPostSync time.Duration
	GenTeardown time.Duration
	RefTeardown time.Duration
	PollTimeout time.Duration
}

// SpeedToBPS converts an Ondatra port speed into bits per second.
func SpeedToBPS(s ondatra.Speed) uint64 {
	const gbps = uint64(1_000_000_000)
	switch s {
	case ondatra.Speed10Gb:
		return 10 * gbps
	case ondatra.Speed25Gb:
		return 25 * gbps
	case ondatra.Speed40Gb:
		return 40 * gbps
	case ondatra.Speed100Gb:
		return 100 * gbps
	case ondatra.Speed400Gb:
		return 400 * gbps
	case ondatra.Speed800Gb:
		return 800 * gbps
	default:
		return uint64(s) * gbps
	}
}

// ParsePortSpeed converts an OpenConfig Ethernet port-speed enum into bits per second.
func ParsePortSpeed(s oc.E_IfEthernet_ETHERNET_SPEED) uint64 {
	return fptest.EthernetSpeedToUint64(s)
}

// ExpectedMinCapabilities computes vendor-aware minimum generator capability expectations.
func ExpectedMinCapabilities(dut *ondatra.DUTDevice, targetSpeedBPS uint64) CapabilityThresholds {
	return computeMinCapabilities(
		targetSpeedBPS,
		deviations.PLQGeneratorCapabilitiesMaxMTU(dut),
		deviations.PLQGeneratorCapabilitiesMaxPPS(dut),
		deviations.PLQGeneratorCapabilitiesMaxBPS(dut),
	)
}

func computeMinCapabilities(targetSpeedBPS uint64, mtuDev uint32, ppsDev, bpsDev uint64) CapabilityThresholds {
	minMTU := defaultMinGeneratorMTU
	if mtuDev != 0 {
		minMTU = uint64(mtuDev)
	}

	minPPS := defaultMinGeneratorPPS
	if ppsDev != 0 {
		minPPS = ppsDev
	}

	minBPS := defaultMinGeneratorBPS
	if targetSpeedBPS > minBPS {
		minBPS = targetSpeedBPS
	}
	if bpsDev != 0 {
		minBPS = bpsDev
	} else if mtuDev != 0 && ppsDev != 0 {
		// Derive the constrained hardware throughput ceiling when MTU and PPS deviations are set.
		derived := minMTU * minPPS * 8
		if derived > 0 && derived < minBPS {
			minBPS = derived
		}
	}

	return CapabilityThresholds{
		MinMTU: minMTU,
		MinPPS: minPPS,
		MinBPS: minBPS,
	}
}

// CalculateRateAndSize computes per-link rate and packet size dynamically from port speed
// and device capabilities while sharing the DUT's generator PPS/BPS budget across concurrentLinks.
func CalculateRateAndSize(portSpeedBPS uint64, caps *plqpb.PacketGeneratorCapabilities, concurrentLinks int, requestedDuration time.Duration) (TrafficParams, error) {
	if err := validateRateInputs(portSpeedBPS, caps, concurrentLinks); err != nil {
		return TrafficParams{}, err
	}

	pktSize := choosePacketSize(caps.GetMinMtu(), caps.GetMaxMtu(), defaultTargetPacketSize)
	bitsPerPkt := uint64(pktSize) * 8

	// Target 10% of port line rate per link, bounded by shared DUT generator budgets.
	targetBPS := portSpeedBPS / 10
	if targetBPS < bitsPerPkt {
		targetBPS = bitsPerPkt
	}

	perLinkMaxPPS := caps.GetMaxPps() / uint64(concurrentLinks)
	if perLinkMaxPPS == 0 {
		perLinkMaxPPS = 1
	}

	wireCapPPS := maxWirePPS(portSpeedBPS, pktSize)
	ratePPS := targetBPS / bitsPerPkt
	if ratePPS > perLinkMaxPPS {
		ratePPS = perLinkMaxPPS
	}
	if wireCapPPS > 0 && ratePPS > wireCapPPS {
		ratePPS = wireCapPPS
	}
	if caps.GetMaxBps() > 0 {
		perLinkMaxBPS := caps.GetMaxBps() / uint64(concurrentLinks)
		capByBPS := perLinkMaxBPS / bitsPerPkt
		if capByBPS < ratePPS {
			ratePPS = capByBPS
		}
	}
	if ratePPS == 0 {
		ratePPS = 1
	}

	return TrafficParams{
		PacketRate: ratePPS,
		PacketSize: pktSize,
		TargetBPS:  ratePPS * bitsPerPkt,
		Duration:   resolveTestDuration(caps, nil, requestedDuration),
	}, nil
}

// CalculateMaxPPSParams computes the packet rate and largest safe packet size that tests
// up to the device's MaxPps capability without exceeding port line rate or generator MaxBps.
func CalculateMaxPPSParams(portSpeedBPS uint64, caps *plqpb.PacketGeneratorCapabilities, requestedDuration time.Duration) (TrafficParams, error) {
	if err := validateRateInputs(portSpeedBPS, caps, 1); err != nil {
		return TrafficParams{}, err
	}

	minMTU := effectiveMinMTU(caps.GetMinMtu(), caps.GetMaxMtu())
	maxMTU := caps.GetMaxMtu()
	if maxMTU > defaultTargetPacketSize {
		maxMTU = defaultTargetPacketSize
	}

	targetPPS := caps.GetMaxPps()
	pktSize := fitPacketSizeForRate(targetPPS, portSpeedBPS, caps.GetMaxBps(), minMTU, maxMTU)
	ratePPS := clampRateToBudgets(targetPPS, pktSize, portSpeedBPS, caps.GetMaxBps())

	return TrafficParams{
		PacketRate: ratePPS,
		PacketSize: pktSize,
		TargetBPS:  ratePPS * uint64(pktSize) * 8,
		Duration:   resolveTestDuration(caps, nil, requestedDuration),
	}, nil
}

// CalculateInjectorParams computes PacketInjector packet count, size, loopback mode, and duration
// from device injector capabilities.
func CalculateInjectorParams(caps *plqpb.PacketInjectorCapabilities, requestedDuration time.Duration) (TrafficParams, error) {
	if caps == nil {
		return TrafficParams{}, fmt.Errorf("packet injector capabilities must not be nil")
	}
	if caps.GetMaxMtu() == 0 {
		return TrafficParams{}, fmt.Errorf("packet injector max_mtu must be > 0")
	}

	pktSize := choosePacketSize(caps.GetMinMtu(), caps.GetMaxMtu(), defaultTargetPacketSize)
	pktCount := clampInjectedPacketCount(caps.GetMinInjectedPackets(), caps.GetMaxInjectedPackets(), defaultInjectedPackets)
	return TrafficParams{
		PacketSize:       pktSize,
		Duration:         resolveDurationFromSample(protoDuration(caps.GetMinSampleInterval()), nil, requestedDuration),
		UseInjector:      true,
		PacketCount:      pktCount,
		InjectorLoopback: chooseInjectorLoopbackMode(caps.GetLoopbackModes()),
	}, nil
}

// CalculateDurations computes synchronized generator and reflector RPCSyncedTiming phases.
// It guarantees generator traffic does not start before reflector setup completes even across mixed vendors.
func CalculateDurations(genCaps *plqpb.PacketGeneratorCapabilities, refCaps *plqpb.ReflectorCapabilities, requestedDuration time.Duration) (Durations, error) {
	return CalculateDurationsForVendors(ondatra.Vendor(0), ondatra.Vendor(0), genCaps, refCaps, requestedDuration)
}

// CalculateDurationsForVendors computes synchronized RPCSyncedTiming phases with vendor-specific
// setup and reflector pre-sync safety floors (e.g. Cisco 180s carrier-delay up hold-off).
func CalculateDurationsForVendors(genVendor, refVendor ondatra.Vendor, genCaps *plqpb.PacketGeneratorCapabilities, refCaps *plqpb.ReflectorCapabilities, requestedDuration time.Duration) (Durations, error) {
	if genCaps == nil {
		return Durations{}, fmt.Errorf("generator capabilities must not be nil")
	}
	setupFloor, refPreSyncFloor := vendorTimingFloors(genVendor, refVendor)
	return calculateDurationsFromTiming(
		protoDuration(genCaps.GetMinSetupDuration()),
		protoDuration(genCaps.GetMinTeardownDuration()),
		protoDuration(genCaps.GetMinSampleInterval()),
		setupFloor,
		refPreSyncFloor,
		refCaps,
		requestedDuration,
	)
}

// CalculateInjectorDurations computes synchronized PacketInjector and reflector RPCSyncedTiming phases.
func CalculateInjectorDurations(injCaps *plqpb.PacketInjectorCapabilities, refCaps *plqpb.ReflectorCapabilities, requestedDuration time.Duration) (Durations, error) {
	return CalculateInjectorDurationsForVendors(ondatra.Vendor(0), ondatra.Vendor(0), injCaps, refCaps, requestedDuration)
}

// CalculateInjectorDurationsForVendors computes synchronized PacketInjector and reflector
// RPCSyncedTiming phases with vendor-specific setup and pre-sync safety floors.
func CalculateInjectorDurationsForVendors(genVendor, refVendor ondatra.Vendor, injCaps *plqpb.PacketInjectorCapabilities, refCaps *plqpb.ReflectorCapabilities, requestedDuration time.Duration) (Durations, error) {
	if injCaps == nil {
		return Durations{}, fmt.Errorf("packet injector capabilities must not be nil")
	}
	setupFloor, refPreSyncFloor := vendorTimingFloors(genVendor, refVendor)
	return calculateDurationsFromTiming(
		protoDuration(injCaps.GetMinSetupDuration()),
		protoDuration(injCaps.GetMinTeardownDuration()),
		protoDuration(injCaps.GetMinSampleInterval()),
		setupFloor,
		refPreSyncFloor,
		refCaps,
		requestedDuration,
	)
}

func vendorTimingFloors(genVendor, refVendor ondatra.Vendor) (setupFloor, refPreSyncFloor time.Duration) {
	if genVendor == ondatra.CISCO || refVendor == ondatra.CISCO {
		return ciscoMinSetupDuration, ciscoRefPreSync
	}
	return 0, 0
}

func calculateDurationsFromTiming(genMinSetup, genMinTeardown, genMinSample, setupFloor, refPreSyncFloor time.Duration, refCaps *plqpb.ReflectorCapabilities, requestedDuration time.Duration) (Durations, error) {
	if refCaps == nil {
		return Durations{}, fmt.Errorf("reflector capabilities must not be nil")
	}

	refMinSetup, refMinTeardown, err := reflectorSetupTeardown(refCaps)
	if err != nil {
		return Durations{}, err
	}

	genSetup := maxDuration(maxDuration(defaultMinGenSetup, genMinSetup), setupFloor)
	refSetup := maxDuration(maxDuration(defaultMinRefSetup, refMinSetup), setupFloor)
	if genSetup > refSetup {
		refSetup = genSetup
	}

	// Reflector starts setup after refPreSync; ensure generator finishes setup at least syncBuffer later.
	refPreSync := refPreSyncFloor
	genPreSync := defaultSyncBuffer
	if needed := refPreSync + refSetup + defaultSyncBuffer - genSetup; needed > genPreSync {
		genPreSync = needed
	}

	testDur := resolveDurationFromSample(genMinSample, refCaps, requestedDuration)
	genPostSync := time.Duration(0)
	refPostSync := defaultMinRefPostSync
	genTeardown := maxDuration(defaultMinGenTeardown, genMinTeardown)
	refTeardown := maxDuration(defaultMinRefTeardown, refMinTeardown)

	genTotal := genPreSync + genSetup + testDur + genPostSync + genTeardown
	refTotal := refPreSync + refSetup + testDur + refPostSync + refTeardown

	return Durations{
		GenPreSync:  genPreSync,
		RefPreSync:  refPreSync,
		GenSetup:    genSetup,
		RefSetup:    refSetup,
		Test:        testDur,
		GenPostSync: genPostSync,
		RefPostSync: refPostSync,
		GenTeardown: genTeardown,
		RefTeardown: refTeardown,
		PollTimeout: maxDuration(genTotal, refTotal) + defaultPollGracePeriod,
	}, nil
}

// ValidateStatsPair verifies completion status, non-zero packet counters, error/drop absence,
// loss tolerance, expected rate delivery, and optional reflector counter consistency.
func ValidateStatsPair(genRes, refRes *plqpb.QualificationResult, expected TrafficParams, validateReflector bool) error {
	if err := validateResultHealth("generator", genRes); err != nil {
		return err
	}
	genSent := genRes.GetPacketsSent()
	genRx := genRes.GetPacketsReceived()
	if genSent == 0 {
		if expected.UseInjector {
			return fmt.Errorf("injector reported 0 packets sent (expected >= %d)", expected.PacketCount)
		}
		return fmt.Errorf("generator reported 0 packets sent (expected rate %d pps)", expected.PacketRate)
	}
	if genRx == 0 {
		return fmt.Errorf("generator reported 0 packets received (sent %d)", genSent)
	}
	if loss := lossPct(genSent, genRx); loss > defaultLossTolerancePct {
		return fmt.Errorf("generator packet loss %.4f%% exceeds tolerance %.4f%% (sent=%d, rx=%d)", loss, defaultLossTolerancePct, genSent, genRx)
	}

	if err := validateDeliveredVolume(genSent, expected); err != nil {
		return err
	}

	if !validateReflector {
		return nil
	}
	if err := validateResultHealth("reflector", refRes); err != nil {
		return err
	}
	refSent := refRes.GetPacketsSent()
	refRx := refRes.GetPacketsReceived()
	if refSent == 0 || refRx == 0 {
		return fmt.Errorf("reflector reported zero counters (sent=%d, rx=%d)", refSent, refRx)
	}
	if loss := lossPct(genSent, refRx); loss > defaultLossTolerancePct {
		return fmt.Errorf("generator->reflector packet loss %.4f%% exceeds %.4f%% (gen_sent=%d, ref_rx=%d)", loss, defaultLossTolerancePct, genSent, refRx)
	}
	if loss := lossPct(refSent, genRx); loss > defaultLossTolerancePct {
		return fmt.Errorf("reflector->generator packet loss %.4f%% exceeds %.4f%% (ref_sent=%d, gen_rx=%d)", loss, defaultLossTolerancePct, refSent, genRx)
	}
	return nil
}

func validateRateInputs(portSpeedBPS uint64, caps *plqpb.PacketGeneratorCapabilities, concurrentLinks int) error {
	if portSpeedBPS == 0 {
		return fmt.Errorf("port speed must be > 0 bps")
	}
	if caps == nil {
		return fmt.Errorf("generator capabilities must not be nil")
	}
	if caps.GetMaxMtu() == 0 {
		return fmt.Errorf("generator max_mtu must be > 0")
	}
	if caps.GetMaxPps() == 0 {
		return fmt.Errorf("generator max_pps must be > 0")
	}
	if concurrentLinks <= 0 {
		return fmt.Errorf("concurrentLinks must be >= 1, got %d", concurrentLinks)
	}
	return nil
}

func choosePacketSize(minCapMTU, maxCapMTU, preferred uint32) uint32 {
	low := effectiveMinMTU(minCapMTU, maxCapMTU)
	high := maxCapMTU
	if preferred < high {
		high = preferred
	}
	if high < low {
		return low
	}
	return high
}

func effectiveMinMTU(minCapMTU, maxCapMTU uint32) uint32 {
	low := defaultMinEthernetFrame
	if minCapMTU > low {
		low = minCapMTU
	}
	if low > maxCapMTU {
		return maxCapMTU
	}
	return low
}

func maxWirePPS(portSpeedBPS uint64, pktSize uint32) uint64 {
	wireBits := (uint64(pktSize) + l1FramingOverheadBytes) * 8
	if wireBits == 0 {
		return 0
	}
	return (portSpeedBPS / 100 * wireHeadroomPercent) / wireBits
}

func fitPacketSizeForRate(ratePPS, portSpeedBPS, maxCapBPS uint64, minMTU, maxMTU uint32) uint32 {
	if ratePPS == 0 {
		return minMTU
	}
	best := maxMTU
	if maxCapBPS > 0 {
		capSize := uint64(maxCapBPS) / (ratePPS * 8)
		if capSize < uint64(best) {
			best = uint32(capSize)
		}
	}
	wireBytesPerPkt := (portSpeedBPS / 100 * wireHeadroomPercent) / (ratePPS * 8)
	if wireBytesPerPkt > l1FramingOverheadBytes {
		wireL2Size := wireBytesPerPkt - l1FramingOverheadBytes
		if wireL2Size < uint64(best) {
			best = uint32(wireL2Size)
		}
	} else {
		best = minMTU
	}
	if best < minMTU {
		return minMTU
	}
	return best
}

func clampRateToBudgets(targetPPS uint64, pktSize uint32, portSpeedBPS, maxCapBPS uint64) uint64 {
	rate := targetPPS
	if wireCap := maxWirePPS(portSpeedBPS, pktSize); wireCap > 0 && rate > wireCap {
		rate = wireCap
	}
	if maxCapBPS > 0 && pktSize > 0 {
		capByBPS := maxCapBPS / (uint64(pktSize) * 8)
		if capByBPS < rate {
			rate = capByBPS
		}
	}
	if rate == 0 {
		return 1
	}
	return rate
}

func resolveTestDuration(genCaps *plqpb.PacketGeneratorCapabilities, refCaps *plqpb.ReflectorCapabilities, requested time.Duration) time.Duration {
	return resolveDurationFromSample(protoDuration(genCaps.GetMinSampleInterval()), refCaps, requested)
}

func resolveDurationFromSample(genMinSample time.Duration, _ *plqpb.ReflectorCapabilities, requested time.Duration) time.Duration {
	dur := defaultQualificationDuration
	if requested > 0 {
		dur = requested
	}
	if sampleMin := genMinSample * 2; sampleMin > dur {
		dur = sampleMin
	}
	return dur
}

func clampInjectedPacketCount(minPkts, maxPkts, preferred uint32) uint32 {
	count := preferred
	if minPkts > count {
		count = minPkts
	}
	if maxPkts > 0 && count > maxPkts {
		count = maxPkts
	}
	if count == 0 {
		return 1
	}
	return count
}

func chooseInjectorLoopbackMode(modes []plqpb.PacketInjectorLoopbackMode) plqpb.PacketInjectorLoopbackMode {
	for _, m := range modes {
		if m == plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_ASIC {
			return m
		}
	}
	for _, m := range modes {
		if m == plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_PMD {
			return m
		}
	}
	return plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_ASIC
}

func reflectorSetupTeardown(refCaps *plqpb.ReflectorCapabilities) (time.Duration, time.Duration, error) {
	if pmd := refCaps.GetPmdLoopback(); pmd != nil && protoDuration(pmd.GetMinSetupDuration()) > 0 {
		return protoDuration(pmd.GetMinSetupDuration()), protoDuration(pmd.GetMinTeardownDuration()), nil
	}
	if asic := refCaps.GetAsicLoopback(); asic != nil && protoDuration(asic.GetMinSetupDuration()) > 0 {
		return protoDuration(asic.GetMinSetupDuration()), protoDuration(asic.GetMinTeardownDuration()), nil
	}
	if pmd := refCaps.GetPmdLoopback(); pmd != nil {
		return protoDuration(pmd.GetMinSetupDuration()), protoDuration(pmd.GetMinTeardownDuration()), nil
	}
	if asic := refCaps.GetAsicLoopback(); asic != nil {
		return protoDuration(asic.GetMinSetupDuration()), protoDuration(asic.GetMinTeardownDuration()), nil
	}
	return 0, 0, fmt.Errorf("reflector capabilities contain neither PmdLoopback nor AsicLoopback timing")
}

func validateResultHealth(role string, res *plqpb.QualificationResult) error {
	if res == nil {
		return fmt.Errorf("%s result is missing", role)
	}
	if code := res.GetStatus().GetCode(); code != int32(codes.OK) {
		return fmt.Errorf("%s qualification status error code=%d msg=%q", role, code, res.GetStatus().GetMessage())
	}
	if res.GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
		return fmt.Errorf("%s state got %v, want COMPLETED", role, res.GetState())
	}
	if res.GetPacketsError() != 0 {
		return fmt.Errorf("%s reported %d packet errors", role, res.GetPacketsError())
	}
	if res.GetPacketsDropped() != 0 {
		return fmt.Errorf("%s reported %d dropped packets", role, res.GetPacketsDropped())
	}
	return nil
}

func validateDeliveredVolume(genSent uint64, expected TrafficParams) error {
	if expected.UseInjector {
		if expected.PacketCount > 0 && genSent < uint64(expected.PacketCount) {
			return fmt.Errorf("injector delivered %d packets, below injected packet_count %d", genSent, expected.PacketCount)
		}
		return nil
	}
	if expected.PacketRate == 0 || expected.Duration <= 0 {
		return nil
	}
	minExpected := uint64(minDeliveredPacketRatio * float64(expected.PacketRate) * expected.Duration.Seconds())
	if minExpected > 0 && genSent < minExpected {
		return fmt.Errorf("generator delivered %d packets over %v, below minimum expected %d (target %d pps)", genSent, expected.Duration, minExpected, expected.PacketRate)
	}
	return nil
}

func lossPct(sent, rx uint64) float64 {
	if sent == 0 {
		return 100.0
	}
	return math.Abs(float64(sent)-float64(rx)) / float64(sent) * 100.0
}

func protoDuration(d interface{ AsDuration() time.Duration }) time.Duration {
	if d == nil {
		return 0
	}
	return d.AsDuration()
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
