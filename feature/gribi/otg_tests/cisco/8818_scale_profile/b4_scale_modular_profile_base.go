package b4_scale_profile_test

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	log_collector "github.com/openconfig/featureprofiles/feature/cisco/performance"
	hautils "github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	util "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	controlcardType      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController     = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController    = oc.Platform_ComponentRedundantRole_SECONDARY
	L1NhPerNHG           = 8
	L1Nhg                = 512
	L2NhPerNHG           = 8
	L2Nhg                = 256
	L3NhPerNHG           = 8
	MaxNhsPerNHG         = 256
	UsableResoucePercent = 90 // 90% of the total resources are usable
	L1Weight             = 16
	L2Weight             = 8
	L3Weight             = 8
)

var (
	flush_before     = flag.Bool("flush_before", true, "Avoid flushing all entries before test by passing flag -flush_before=false")
	flush_after      = flag.Bool("flush_after", true, "Avoid flushing all entries after test by passing flag -flush_after=false")
	debugCommandYaml = flag.String("debug_command_yaml", "", "Path to debug command YAML file")
	logDir           = flag.String("logDir", "", "Firex path to copy the logs after each test case")
	GlobalIDPool     = NewIDPool(20000)
	tunnelDestIPs    = iputil.GenerateIPs(V4TunnelIPBlock, encapNhCount)
	vipIPs           = iputil.GenerateIPs(V4VIPIPBlock, L1Nhg)
	vipFrr1IPs       = iputil.GenerateIPs(VipFrr1IPBlock, L1Nhg)
	bConfig          = newBaseConfig()
	gArgs            *testArgs
	triggers         = []Trigger{}
)

// Define a slice of test triggers with a duration for each
type Trigger struct {
	name                  string
	fn                    func(ctx context.Context, t *testing.T)
	duration              time.Duration
	reprogrammingRequired bool
	reconnectClient       bool
}

type baseConfig struct {
	m          sync.Mutex
	configured bool
	// modified   bool
}

func newBaseConfig() *baseConfig {
	return &baseConfig{}
}

func (b *baseConfig) isConfigured() bool {
	b.m.Lock()
	defer b.m.Unlock()
	return b.configured
}
func (b *baseConfig) setConfigured(c bool) {
	b.m.Lock()
	defer b.m.Unlock()
	b.configured = c
}

// func (b *baseConfig) isModified() bool {
// 	b.m.Lock()
// 	defer b.m.Unlock()
// 	return b.modified
// }
// func (b *baseConfig) setModified(c bool) {
// 	b.m.Lock()
// 	defer b.m.Unlock()
// 	b.modified = c
// }

// PairedEntries holds NHs, NHGs and IPv4 entries for the VRF.
type PairedEntries struct {
	NHs        []fluent.GRIBIEntry
	NHGs       []fluent.GRIBIEntry
	V4Entries  []fluent.GRIBIEntry
	V6Entries  []fluent.GRIBIEntry
	V4Prefixes []string
	V6Prefixes []string
}

func NewPairedEntry() *PairedEntries {
	return &PairedEntries{}
}

// GetFirstIPv4PrefixAndCount returns the first IPv4 prefix and the count of IPv4 prefixes.
func (p *PairedEntries) GetFirstIPv4PrefixAndCount() (string, int) {
	if len(p.V4Prefixes) > 0 {
		return p.V4Prefixes[0], len(p.V4Prefixes)
	}
	return "", 0
}

// GetFirstIPv4PrefixAndCount returns the first IPv4 prefix and the count of IPv4 prefixes.
func (p *PairedEntries) GetFirstIPv6PrefixAndCount() (string, int) {
	if len(p.V6Prefixes) > 0 {
		return p.V6Prefixes[0], len(p.V6Prefixes)
	}
	return "", 0
}

type tunTypes struct {
	location string
	tunType  []string
}

type GribiProfile struct {
	PrimaryLevel1          *routesParam
	PrimaryLevel2          *routesParam
	PrimaryLevel3A         *routesParam
	PrimaryLevel3B         *routesParam
	PrimaryLevel3C         *routesParam
	PrimaryLevel3D         *routesParam
	Frr1Level1             *routesParam
	Frr1Level2             *routesParam
	DecapWan               *routesParam
	DecapWanVar            *routesParam
	backUpFluentEntries    []fluent.GRIBIEntry
	batches                int
	usedBatches            *coniguredBatches
	PrimaryL1Entries       []PairedEntries
	PrimaryL2Entries       []PairedEntries
	Frr1L1Entries          []PairedEntries
	Frr1L2Entries          []PairedEntries
	EncapEntriesA          []PairedEntries
	EncapEntriesB          []PairedEntries
	EncapEntriesC          []PairedEntries
	EncapEntriesD          []PairedEntries
	DecapWanEntries        []PairedEntries
	DecapWanVarEntries     []PairedEntries
	ConmbinedPairedEntries [][]fluent.GRIBIEntry
	useBackups             bool
	measurePerf            *tunTypes
}

func NewGribiProfile(t *testing.T, batches int, frr1bkp bool, frr2bkp bool, dut *ondatra.DUTDevice, rp ...*routesParam) *GribiProfile {
	gp := &GribiProfile{}
	if frr1bkp || frr2bkp {
		gp.useBackups = true
	} else {
		gp.useBackups = false
	}

	gp.batches = batches
	gp.backUpFluentEntries = []fluent.GRIBIEntry{}

	nhID := GlobalIDPool.NextNHID()
	nhgDecapToDefault := GlobalIDPool.NextNHGID()
	gp.backUpFluentEntries = append(gp.backUpFluentEntries,
		fluent.NextHopEntry().WithIndex(nhID).WithDecapsulateHeader(fluent.IPinIP).WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	gp.backUpFluentEntries = append(gp.backUpFluentEntries,
		fluent.NextHopGroupEntry().WithID(nhgDecapToDefault).AddNextHop(nhID, 1).WithNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	// backup used in FRR1 case
	nhgRedirectToVrfR := GlobalIDPool.NextNHGID()
	nhID = GlobalIDPool.NextNHID()
	// build backup NHG and NH.
	gp.backUpFluentEntries = append(gp.backUpFluentEntries,
		fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).WithNextHopNetworkInstance(VRFR),
	)
	gp.backUpFluentEntries = append(gp.backUpFluentEntries,
		fluent.NextHopGroupEntry().WithID(nhgRedirectToVrfR).AddNextHop(nhID, 1).WithNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	for _, p := range rp {
		if p.segment == "PrimaryLevel1" {
			if p.ipEntries == nil {
				p.ipEntries = vipIPs
			}
			if p.prefixVRF == "" {
				p.prefixVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "default"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = L1Nhg
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = L1NhPerNHG
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L1Weight, L1NhPerNHG)
			}
			gp.PrimaryLevel1 = p
			gp.PrimaryL1Entries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "PrimaryLevel2" {
			// ipEntries:     tunnelDestIPs, // 1600 tunnel prefixes - will be 6800 in final
			// prefixVRF:     vrfTransit,
			// nextHops:      vipIPs, // VIP addresses
			// nextHopVRF:    deviations.DefaultNetworkInstance(dut),
			// nextHopType:   "default",
			// numUniqueNHGs: L2Nhg, // 256
			// numNHPerNHG:   2,     // each prefix uses a NHG with 2 NHs
			// nextHopWeight: generateNextHopWeights(256, L2NhPerNHG),
			// backupNHG:     int(nhgRedirectToVrfR),
			if p.ipEntries == nil {
				p.ipEntries = tunnelDestIPs
			}
			if p.prefixVRF == "" {
				p.prefixVRF = vrfTransit
			}
			if p.nextHops == nil {
				p.nextHops = vipIPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "default"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = L2Nhg
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 2
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L2Weight, L2NhPerNHG)
			}
			if p.backupNHG == 0 && frr1bkp {
				p.backupNHG = int(nhgRedirectToVrfR)
			}
			gp.PrimaryLevel2 = p
			gp.PrimaryL2Entries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "PrimaryLevel3A" {
			// ipEntries:     encapVrfAIPv4Enries,
			// ipv6Entries:   encapVrfAIPv6Enries,
			// prefixVRF:     vrfEncapA,
			// nextHops:      tunnelDestIPs,
			// nextHopVRF:    vrfTransit,
			// nextHopType:   "encap",
			// startNHIndex:  lastNhIndex + 1, // not used
			// numUniqueNHGs: 200,             //encapNhgcount,
			// numNHPerNHG:   8,
			// nextHopWeight: generateNextHopWeights(16, 8),
			// tunnelSrcIP:   ipv4OuterSrc111,
			if p.ipEntries == nil {
				p.ipEntries = encapVrfAIPv4Enries
			}
			if p.ipv6Entries == nil {
				p.ipv6Entries = encapVrfAIPv6Enries
			}
			if p.prefixVRF == "" {
				p.prefixVRF = vrfEncapA
			}
			if p.nextHops == nil {
				p.nextHops = tunnelDestIPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = vrfTransit
			}
			if p.nextHopType == "" {
				p.nextHopType = "encap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 200
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 8
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L3Weight, L3NhPerNHG)
			}
			if p.tunnelSrcIP == "" {
				p.tunnelSrcIP = ipv4OuterSrc111
			}
			gp.PrimaryLevel3A = p
			gp.EncapEntriesA = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "PrimaryLevel3B" {
			// ipEntries:     encapVrfBIPv4Enries,
			// ipv6Entries:   encapVrfBIPv6Enries,
			// prefixVRF:     vrfEncapB,
			// nextHops:      tunnelDestIPs,
			// nextHopVRF:    vrfTransit,
			// nextHopType:   "encap",
			// numUniqueNHGs: 200, //encapNhgcount,
			// numNHPerNHG:   8,
			// nextHopWeight: generateNextHopWeights(16, 8),
			// tunnelSrcIP:   ipv4OuterSrc111,
			if p.ipEntries == nil {
				p.ipEntries = encapVrfBIPv4Enries
			}
			if p.ipv6Entries == nil {
				p.ipv6Entries = encapVrfBIPv6Enries
			}
			if p.prefixVRF == "" {
				p.prefixVRF = vrfEncapB
			}
			if p.nextHops == nil {
				p.nextHops = tunnelDestIPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = vrfTransit
			}
			if p.nextHopType == "" {
				p.nextHopType = "encap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 200
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 8
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L3Weight, L3NhPerNHG)
			}
			if p.tunnelSrcIP == "" {
				p.tunnelSrcIP = ipv4OuterSrc111
			}
			gp.PrimaryLevel3B = p
			gp.EncapEntriesB = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "PrimaryLevel3C" {
			// ipEntries:     encapVrfCIPv4Enries,
			// ipv6Entries:   encapVrfCIPv6Enries,
			// prefixVRF:     vrfEncapC,
			// nextHops:      tunnelDestIPs,
			// nextHopVRF:    vrfTransit,
			// nextHopType:   "encap",
			// numUniqueNHGs: 200, //encapNhgcount,
			// numNHPerNHG:   8,
			// nextHopWeight: generateNextHopWeights(16, 8),
			// tunnelSrcIP:   ipv4OuterSrc222,
			if p.ipEntries == nil {
				p.ipEntries = encapVrfCIPv4Enries
			}
			if p.ipv6Entries == nil {
				p.ipv6Entries = encapVrfCIPv6Enries
			}
			if p.prefixVRF == "" {
				p.prefixVRF = vrfEncapC
			}
			if p.nextHops == nil {
				p.nextHops = tunnelDestIPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = vrfTransit
			}
			if p.nextHopType == "" {
				p.nextHopType = "encap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 200
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 8
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L3Weight, L3NhPerNHG)
			}
			if p.tunnelSrcIP == "" {
				p.tunnelSrcIP = ipv4OuterSrc222
			}
			gp.PrimaryLevel3C = p
			gp.EncapEntriesC = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "PrimaryLevel3D" {
			// ipEntries:     encapVrfDIPv4Enries,
			// ipv6Entries:   encapVrfDIPv6Enries,
			// prefixVRF:     vrfEncapD,
			// nextHops:      tunnelDestIPs,
			// nextHopVRF:    vrfTransit,
			// nextHopType:   "encap",
			// numUniqueNHGs: 200, //encapNhgcount,
			// numNHPerNHG:   8,
			// nextHopWeight: generateNextHopWeights(16, 8),
			// tunnelSrcIP:   ipv4OuterSrc222,
			if p.ipEntries == nil {
				p.ipEntries = encapVrfDIPv4Enries
			}
			if p.ipv6Entries == nil {
				p.ipv6Entries = encapVrfDIPv6Enries
			}
			if p.prefixVRF == "" {
				p.prefixVRF = vrfEncapD
			}
			if p.nextHops == nil {
				p.nextHops = tunnelDestIPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = vrfTransit
			}
			if p.nextHopType == "" {
				p.nextHopType = "encap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 200
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 8
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L3Weight, L3NhPerNHG)
			}
			if p.tunnelSrcIP == "" {
				p.tunnelSrcIP = ipv4OuterSrc222
			}
			gp.PrimaryLevel3D = p
			gp.EncapEntriesD = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "Frr1Level1" {
			// ipEntries:     vipFrr1IPs, // 512 VIP prefixes
			// prefixVRF:     deviations.DefaultNetworkInstance(dut),
			// nextHops:      peerNHIP, // peer or otg prefixes
			// nextHopVRF:    deviations.DefaultNetworkInstance(dut),
			// nextHopType:   "default",
			// numUniqueNHGs: L1Nhg,      // 512
			// numNHPerNHG:   L1NhPerNHG, //8
			// nextHopWeight: generateNextHopWeights(64, 8),
			if p.ipEntries == nil {
				p.ipEntries = vipFrr1IPs
			}
			if p.prefixVRF == "" {
				p.prefixVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHops == nil {
				// p.nextHops = peerNHIP
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "default"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = L1Nhg
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = L1NhPerNHG
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L1Weight, L1NhPerNHG)
			}
			gp.Frr1Level1 = p
			gp.Frr1L1Entries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "Frr1Level2" {
			// ipEntries:     tunnelDestIPs, // 1600 tunnel prefixes - will be 6800 in final
			// prefixVRF:     VRFR,
			// nextHops:      vipFrr1IPs, // VIP addresses. Tunnel Dest IPs are same as VIPs
			// nextHopVRF:    deviations.DefaultNetworkInstance(dut),
			// nextHopType:   "decapEncap",
			// numUniqueNHGs: L2Nhg,      // 256
			// numNHPerNHG:   L2NhPerNHG, // 8
			// nextHopWeight: generateNextHopWeights(256, L2NhPerNHG),
			// backupNHG:     int(nhgDecapToDefault),
			// tunnelSrcIP:   ipv4OuterSrc222,
			if p.ipEntries == nil {
				p.ipEntries = tunnelDestIPs
			}
			if p.prefixVRF == "" {
				p.prefixVRF = VRFR
			}
			if p.nextHops == nil {
				p.nextHops = vipFrr1IPs
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "decapEncap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = L2Nhg
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = L2NhPerNHG
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(L2Weight, L2NhPerNHG)
			}
			if p.backupNHG == 0 && frr2bkp {
				p.backupNHG = int(nhgDecapToDefault)
			}
			if p.tunnelSrcIP == "" {
				p.tunnelSrcIP = ipv4OuterSrc222
			}
			gp.Frr1Level2 = p
			gp.Frr1L2Entries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "DecapWan" {
			// ipEntries:     iputil.GenerateIPs(IPBlockDecap, decapIPv4ScaleCount),
			// prefixVRF:     niDecapTeVrf,
			// nextHops:      []string{}, // not used for decap
			// nextHopVRF:    deviations.DefaultNetworkInstance(dut),
			// nextHopType:   "decap",
			// numUniqueNHGs: 1000, //encapNhgcount,
			// numNHPerNHG:   1,
			// nextHopWeight: generateNextHopWeights(1, 1),
			if p.ipEntries == nil {
				p.ipEntries = iputil.GenerateIPs(IPBlockDecap, decapIPv4ScaleCount)
			}
			if p.prefixVRF == "" {
				p.prefixVRF = niDecapTeVrf
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "decap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 1000
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 1
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(1, 1)
			}
			gp.DecapWan = p
			gp.DecapWanEntries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
		if p.segment == "DecapWanVar" {
			// ipEntries:     getVariableLenSubnets(12, "102.51.100.1/22", "107.51.105.1/24", "112.51.110.1/26", "117.51.115.1/28"),
			// addrPerSubnet: 1,
			// prefixVRF:     niDecapTeVrf,
			// nextHops:      []string{}, // not used for decap
			// nextHopVRF:    deviations.DefaultNetworkInstance(dut),
			// nextHopType:   "decap",
			// numUniqueNHGs: 48,
			// numNHPerNHG:   1,
			// nextHopWeight: generateNextHopWeights(1, 1),
			if p.ipEntries == nil {
				p.ipEntries = getVariableLenSubnets(12, "102.51.100.1/22", "107.51.105.1/24", "112.51.110.1/26", "117.51.115.1/28")
			}
			if p.addrPerSubnet == 0 {
				p.addrPerSubnet = 1
			}
			if p.prefixVRF == "" {
				p.prefixVRF = niDecapTeVrf
			}
			if p.nextHopVRF == "" {
				p.nextHopVRF = deviations.DefaultNetworkInstance(dut)
			}
			if p.nextHopType == "" {
				p.nextHopType = "decap"
			}
			if p.numUniqueNHGs == 0 {
				p.numUniqueNHGs = 48
			}
			if p.numNHPerNHG == 0 {
				p.numNHPerNHG = 1
			}
			if p.nextHopWeight == nil {
				p.nextHopWeight = generateNextHopWeights(1, 1)
			}
			gp.DecapWanVar = p
			gp.DecapWanVarEntries = GetFibSegmentGribiEntries(t, p, dut, batches)
		}
	}
	gp.ConmbinedPairedEntries = CombinePairedEntries(t, dut, gp.batches, gp.GetNonEmptyRoutesParams()...)
	gp.usedBatches = &coniguredBatches{conBatches: []int{}}
	return gp
}

func (gp *GribiProfile) pushBatchConfig(t *testing.T, tcArgs *testArgs, batchSet []int) {
	if len(batchSet) > gp.batches {
		t.Error("batchSet is greater than total configuration batches")
	} else {
		entries := []fluent.GRIBIEntry{}
		for _, batch := range batchSet {
			entries = append(entries, gp.ConmbinedPairedEntries[batch]...)
		}
		if gp.useBackups {
			if gp.measurePerf != nil {
				for _, tn := range gp.measurePerf.tunType {
					clearOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
					// clearOfaPerformance(t, tcArgs.dut, "iptnlencap", "0/0/CPU0")
					// clearOfaPerformance(t, tcArgs.dut, "iptnldecap", "0/0/CPU0")
				}
			}
			t.Logf("Programming backup entries")
			tcArgs.client.Modify().AddEntry(t, gp.backUpFluentEntries...)
			if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, 1*time.Minute); err != nil {
				t.Fatalf("Could not program entries, got err: %v", err)
			}
			if gp.measurePerf != nil {
				time.Sleep(5 * time.Second) // wait for 5 seconds to get the performance data to get updated
				for _, tn := range gp.measurePerf.tunType {
					getOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
				}
			}
		}
		// Program the entries
		if gp.measurePerf != nil {
			for _, tn := range gp.measurePerf.tunType {
				clearOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
			}
		}
		t.Logf("Programming %d entries", len(entries))
		tcArgs.client.Modify().AddEntry(t, entries...)
		if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, aftProgTimeout); err != nil {
			t.Fatalf("Could not program entries, got err: %v", err)
		}
		if gp.measurePerf != nil {
			time.Sleep(5 * time.Second) // wait for 5 seconds to get the performance data to get updated
			for _, tn := range gp.measurePerf.tunType {
				getOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
			}
		}
		gp.usedBatches.useBatch(batchSet)
	}
}

func (gp *GribiProfile) DeleteBatchConfig(t *testing.T, tcArgs *testArgs, batchSet []int) {
	if len(batchSet) > gp.batches {
		t.Error("batchSet is greater than total configuration batches")
	} else {
		// only program the first batch
		entries := []fluent.GRIBIEntry{}
		for _, batch := range batchSet {
			entries = append(entries, gp.ConmbinedPairedEntries[batch]...)
		}
		if gp.measurePerf != nil {
			for _, tn := range gp.measurePerf.tunType {
				clearOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
			}
		}
		// Program the entries
		t.Logf("Deleting %d entries", len(entries))
		tcArgs.client.Modify().DeleteEntry(t, entries...)
		if gp.measurePerf != nil {
			for _, tn := range gp.measurePerf.tunType {
				getOfaPerformance(t, tcArgs.dut, tn, gp.measurePerf.location)
			}
		}
		gp.usedBatches.freeBatch(batchSet)
	}
}

func (gp *GribiProfile) GetNonEmptyRoutesParams() []*routesParam {
	var nonEmptyParams []*routesParam

	// Check each field in the GribiProfile for non-nil and non-empty ipEntries
	if gp.PrimaryLevel1 != nil && len(gp.PrimaryLevel1.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel1)
	}
	if gp.PrimaryLevel2 != nil && len(gp.PrimaryLevel2.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel2)
	}
	if gp.PrimaryLevel3A != nil && len(gp.PrimaryLevel3A.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel3A)
	}
	if gp.PrimaryLevel3B != nil && len(gp.PrimaryLevel3B.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel3B)
	}
	if gp.PrimaryLevel3C != nil && len(gp.PrimaryLevel3C.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel3C)
	}
	if gp.PrimaryLevel3D != nil && len(gp.PrimaryLevel3D.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.PrimaryLevel3D)
	}
	if gp.Frr1Level1 != nil && len(gp.Frr1Level1.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.Frr1Level1)
	}
	if gp.Frr1Level2 != nil && len(gp.Frr1Level2.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.Frr1Level2)
	}
	if gp.DecapWan != nil && len(gp.DecapWan.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.DecapWan)
	}
	if gp.DecapWanVar != nil && len(gp.DecapWanVar.ipEntries) > 0 {
		nonEmptyParams = append(nonEmptyParams, gp.DecapWanVar)
	}

	return nonEmptyParams
}

func CombinePairedEntries(t *testing.T, dut *ondatra.DUTDevice, batchCount int, routeParams ...*routesParam) [][]fluent.GRIBIEntry {
	// Create a result slice with the same number of batches as the batchCount
	result := make([][]fluent.GRIBIEntry, batchCount)

	// Initialize counters for combined entries
	totalNHs, totalNHGs, totalV4s, totalV6s := 0, 0, 0, 0

	// Iterate over each routeParam
	for _, params := range routeParams {
		// Get the PairedEntries for the current routeParam
		pairedEntries := GetFibSegmentGribiEntries(t, params, dut, batchCount)

		// Initialize counters for this routeParam
		paramNHs, paramNHGs, paramV4s, paramV6s := 0, 0, 0, 0

		// Combine the entries into the result batches
		for i := 0; i < batchCount; i++ {
			if i < len(pairedEntries) {
				// Count entries for this batch
				batchNHs := len(pairedEntries[i].NHs)
				batchNHGs := len(pairedEntries[i].NHGs)
				batchV4s := len(pairedEntries[i].V4Entries)
				batchV6s := len(pairedEntries[i].V6Entries)

				// Update counters for this routeParam
				paramNHs += batchNHs
				paramNHGs += batchNHGs
				paramV4s += batchV4s
				paramV6s += batchV6s

				// Update combined counters
				totalNHs += batchNHs
				totalNHGs += batchNHGs
				totalV4s += batchV4s
				totalV6s += batchV6s

				// Combine all entries (NHs, NHGs, V4Entries, V6Entries) into a single slice for this batch
				combinedEntries := append([]fluent.GRIBIEntry{}, pairedEntries[i].NHs...)
				combinedEntries = append(combinedEntries, pairedEntries[i].NHGs...)
				combinedEntries = append(combinedEntries, pairedEntries[i].V4Entries...)
				combinedEntries = append(combinedEntries, pairedEntries[i].V6Entries...)

				// Add the combined entries to the result
				result[i] = append(result[i], combinedEntries...)
			}
		}

		// Log counts for this fib chain segment
		t.Logf("FibSegment %v: Total NHs: %d, NHGs: %d, V4Entries: %d, V6Entries: %d", params.segment, paramNHs, paramNHGs, paramV4s, paramV6s)
	}

	// Log combined counts for all fib chain segments
	t.Logf("Combined: Total NHs: %d, NHGs: %d, V4Entries: %d, V6Entries: %d", totalNHs, totalNHGs, totalV4s, totalV6s)

	return result
}

type coniguredBatches struct {
	m          sync.Mutex
	conBatches []int
}

func (c *coniguredBatches) useBatch(batchSet []int) {
	c.m.Lock()
	defer c.m.Unlock()
	for _, b := range batchSet {
		exists := false
		for _, existing := range c.conBatches {
			if existing == b {
				exists = true
				break
			}
		}
		if !exists {
			c.conBatches = append(c.conBatches, b)
		}
	}
}

// func (c *coniguredBatches) getBatches() []int {
// 	c.m.Lock()
// 	defer c.m.Unlock()
// 	return c.conBatches
// }

func (c *coniguredBatches) freeBatch(batchSet []int) {
	c.m.Lock()
	defer c.m.Unlock()
	for _, b := range batchSet {
		for i, existing := range c.conBatches {
			if existing == b {
				c.conBatches = append(c.conBatches[:i], c.conBatches[i+1:]...)
				break
			}
		}
	}
}

type DecapFlowAttr struct {
	outerIP    []string
	innerV4Dst []string
	innerV6Dst []string
	dscp       uint32
}

type EncapFlowAttr struct {
	outerV4Dst []string
	outerV6Dst []string
	dscp       uint32
}

func testEncapTrafficFlows(t *testing.T, tcArgs *testArgs, gp *GribiProfile, batchSet []int, opts ...*ConvOptions) {
	flows := []gosnappi.Flow{}
	for _, batch := range batchSet {
		if gp.EncapEntriesA != nil && len(gp.EncapEntriesA[batch].V4Prefixes) > 0 {
			flows = append(flows, getEncapFlowsForBatch(batch, "encpA", &EncapFlowAttr{gp.EncapEntriesA[batch].V4Prefixes, gp.EncapEntriesA[batch].V6Prefixes, dscpEncapA1})...)
		}
		if gp.EncapEntriesB != nil && len(gp.EncapEntriesB[batch].V4Prefixes) > 0 {
			flows = append(flows, getEncapFlowsForBatch(batch, "encpB", &EncapFlowAttr{gp.EncapEntriesB[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapB1})...)
		}
		if gp.EncapEntriesC != nil && len(gp.EncapEntriesC[batch].V4Prefixes) > 0 {
			flows = append(flows, getEncapFlowsForBatch(batch, "encpC", &EncapFlowAttr{gp.EncapEntriesC[batch].V4Prefixes, gp.EncapEntriesC[batch].V6Prefixes, dscpEncapC1})...)
		}
		if gp.EncapEntriesD != nil && len(gp.EncapEntriesD[batch].V4Prefixes) > 0 {
			flows = append(flows, getEncapFlowsForBatch(batch, "encpD", &EncapFlowAttr{gp.EncapEntriesD[batch].V4Prefixes, gp.EncapEntriesD[batch].V6Prefixes, dscpEncapD1})...)
		}
	}
	validateTrafficFlows(t, tcArgs, flows, false, true)
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.measureConvergence {

				t.Run("Convergence with first frr & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRFirst: "1"})
				})
				t.Run("Convergence with two frrs & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
				t.Run("Convergence with forwarding viable & recovery", func(t *testing.T) {
					doBatchconfig(t, pathInfo.PrimaryInterface, "", "viable")
					doBatchconfig(t, pathInfo.BackupInterface, "", "viable")

					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
			}
		}
	}
}

func testDecapTrafficFlows(t *testing.T, tcArgs *testArgs, gp *GribiProfile, batchSet []int, opts ...*ConvOptions) {
	flows := []gosnappi.Flow{}
	for _, batch := range batchSet {
		if gp.DecapWanEntries != nil && len(gp.DecapWanEntries[batch].V4Prefixes) > 0 {
			if gp.EncapEntriesA != nil && len(gp.EncapEntriesA[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatchUsingFlowCount(batch, "dcapF",
					&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesA[batch].V4Prefixes, gp.EncapEntriesA[batch].V6Prefixes, dscpEncapA1})...)
			}
			if gp.EncapEntriesB != nil && len(gp.EncapEntriesB[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatchUsingFlowCount(batch, "dcapF",
					&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesB[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapB1})...)
			}
			if gp.EncapEntriesC != nil && len(gp.EncapEntriesC[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatchUsingFlowCount(batch, "dcapF",
					&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesC[batch].V4Prefixes, gp.EncapEntriesC[batch].V6Prefixes, dscpEncapC1})...)
			}
			if gp.EncapEntriesD != nil && len(gp.EncapEntriesD[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatchUsingFlowCount(batch, "dcapF",
					&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesD[batch].V4Prefixes, gp.EncapEntriesD[batch].V6Prefixes, dscpEncapD1})...)
			}
		}

		if gp.DecapWanVarEntries != nil && len(gp.DecapWanVarEntries[batch].V4Prefixes) > 0 {
			if gp.EncapEntriesA != nil && len(gp.EncapEntriesA[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
					&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesA[batch].V4Prefixes, gp.EncapEntriesA[batch].V6Prefixes, dscpEncapA1})...)
			}
			if gp.EncapEntriesB != nil && len(gp.EncapEntriesB[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
					&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesB[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapB1})...)
			}
			if gp.EncapEntriesC != nil && len(gp.EncapEntriesC[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
					&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesC[batch].V4Prefixes, gp.EncapEntriesC[batch].V6Prefixes, dscpEncapC1})...)
			}
			if gp.EncapEntriesD != nil && len(gp.EncapEntriesD[batch].V4Prefixes) > 0 {
				flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
					&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesD[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapD1})...)
			}
		}
	}
	validateTrafficFlows(t, tcArgs, flows, false, true)

	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.measureConvergence {

				t.Run("Convergence with first frr & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRFirst: "1"})
				})
				t.Run("Convergence with two frrs & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
				t.Run("Convergence with forwarding viable & recovery", func(t *testing.T) {
					doBatchconfig(t, pathInfo.PrimaryInterface, "", "viable")
					doBatchconfig(t, pathInfo.BackupInterface, "", "viable")

					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
			}
		}
	}
}

func testDecapTrafficFlowsForEncap(t *testing.T, tcArgs *testArgs, gp *GribiProfile, batchSet []int, encapType []string, opts ...*ConvOptions) {
	flows := []gosnappi.Flow{}
	for _, batch := range batchSet {
		if gp.DecapWanEntries != nil && len(gp.DecapWanEntries[batch].V4Prefixes) > 0 {
			for _, encap := range encapType {
				t.Logf("Adding flow for /32 prefix decap traffic for batch %d and encap %s", batch, encap)
				switch encap {
				case "A":
					if gp.EncapEntriesA != nil && len(gp.EncapEntriesA[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapF",
							&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesA[batch].V4Prefixes, gp.EncapEntriesA[batch].V6Prefixes, dscpEncapA1})...)
					}
				case "B":
					if gp.EncapEntriesB != nil && len(gp.EncapEntriesB[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapF",
							&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesB[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapB1})...)
					}
				case "C":
					if gp.EncapEntriesC != nil && len(gp.EncapEntriesC[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapF",
							&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesC[batch].V4Prefixes, gp.EncapEntriesC[batch].V6Prefixes, dscpEncapC1})...)
					}
				case "D":
					if gp.EncapEntriesD != nil && len(gp.EncapEntriesD[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapF",
							&DecapFlowAttr{gp.DecapWanEntries[batch].V4Prefixes, gp.EncapEntriesD[batch].V4Prefixes, gp.EncapEntriesD[batch].V6Prefixes, dscpEncapD1})...)
					}
				}
			}
		}
	}
	validateTrafficFlows(t, tcArgs, flows, false, true)
	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.measureConvergence {

				t.Run("Convergence with first frr & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRFirst: "1"})
				})
				t.Run("Convergence with two frrs & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
				t.Run("Convergence with forwarding viable & recovery", func(t *testing.T) {
					doBatchconfig(t, pathInfo.PrimaryInterface, "", "viable")
					doBatchconfig(t, pathInfo.BackupInterface, "", "viable")

					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
			}
		}
	}
}

func testDecapTrafficFlowsForVariablePrefix(t *testing.T, tcArgs *testArgs, gp *GribiProfile, batchSet []int, encapType []string, opts ...*ConvOptions) {
	flows := []gosnappi.Flow{}
	for _, batch := range batchSet {
		if gp.DecapWanVarEntries != nil && len(gp.DecapWanVarEntries[batch].V4Prefixes) > 0 {
			for _, encap := range encapType {
				t.Logf("Adding flow for variable prefix decap traffic for batch %d and encap %s", batch, encap)
				switch encap {
				case "A":
					if gp.EncapEntriesA != nil && len(gp.EncapEntriesA[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
							&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesA[batch].V4Prefixes, gp.EncapEntriesA[batch].V6Prefixes, dscpEncapA1})...)
					}
				case "B":
					if gp.EncapEntriesB != nil && len(gp.EncapEntriesB[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
							&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesB[batch].V4Prefixes, gp.EncapEntriesB[batch].V6Prefixes, dscpEncapB1})...)
					}
				case "C":
					if gp.EncapEntriesC != nil && len(gp.EncapEntriesC[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
							&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesC[batch].V4Prefixes, gp.EncapEntriesC[batch].V6Prefixes, dscpEncapC1})...)
					}
				case "D":
					if gp.EncapEntriesD != nil && len(gp.EncapEntriesD[batch].V4Prefixes) > 0 {
						flows = append(flows, getDecapFlowsForBatch(batch, "dcapV",
							&DecapFlowAttr{gp.DecapWanVarEntries[batch].V4Prefixes, gp.EncapEntriesD[batch].V4Prefixes, gp.EncapEntriesD[batch].V6Prefixes, dscpEncapD1})...)
					}
				}
			}
		}
	}
	validateTrafficFlows(t, tcArgs, flows, false, true)

	if len(opts) != 0 {
		for _, opt := range opts {
			if opt.measureConvergence {

				t.Run("Convergence with first frr & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRFirst: "1"})
				})
				t.Run("Convergence with two frrs & recovery", func(t *testing.T) {
					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
				t.Run("Convergence with forwarding viable & recovery", func(t *testing.T) {
					doBatchconfig(t, pathInfo.PrimaryInterface, "", "viable")
					doBatchconfig(t, pathInfo.BackupInterface, "", "viable")

					validateTrafficFlows(t, tcArgs, flows, false, true, &ConvOptions{convFRRSecond: "2"})
				})
			}
		}
	}
}

// getOuterSrcForDscp returns the outer source IP address for a given DSCP value
func getOuterSrcForDscp(dscp uint32) string {
	switch dscp {
	case dscpEncapA1:
		return ipv4OuterSrc111
	case dscpEncapA2:
		return ipv4OuterSrc222
	case dscpEncapB1:
		return ipv4OuterSrc111
	case dscpEncapB2:
		return ipv4OuterSrc222
	case dscpEncapC1:
		return ipv4OuterSrc111
	case dscpEncapC2:
		return ipv4OuterSrc222
	case dscpEncapD1:
		return ipv4OuterSrc111
	case dscpEncapD2:
		return ipv4OuterSrc222
	default:
		return ipv4OuterSrc111
	}
}

// dscpToString returns a string representation of a DSCP value
func dscpToString(dscp uint32) string {
	switch dscp {
	case dscpEncapA1:
		return "dscpEncapA1"
	case dscpEncapA2:
		return "dscpEncapA2"
	case dscpEncapB1:
		return "dscpEncapB1"
	case dscpEncapB2:
		return "dscpEncapB2"
	case dscpEncapC1:
		return "dscpEncapC1"
	case dscpEncapC2:
		return "dscpEncapC2"
	case dscpEncapD1:
		return "dscpEncapD1"
	case dscpEncapD2:
		return "dscpEncapD2"
	default:
		return "dscpEncapA1"
	}
}

// getDecapFlowsForBatch creates decap flows for a given batch
func getDecapFlowsForBatchUsingFlowCount(batch int, name string, dfa ...*DecapFlowAttr) []gosnappi.Flow {

	var dInV4 = trafficflowAttr{
		withInnerHeader: true, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName2 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc2.MAC,                     // source MAC address
		dstMac:          dutSrc2.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}

	flows := []gosnappi.Flow{}

	for i, f := range dfa {
		j := i * 2
		// define outer header
		dInV4.useOuterFlowIncrement = true

		dInV4.outerDst = f.outerIP // first IP will be used as seed for outer IP
		dInV4.outerFlowCount = uint32(len(f.outerIP))
		dInV4.outerSrc = getOuterSrcForDscp(f.dscp)

		// common attribute for inner flows
		dInV4.useInnerFlowIncrement = true

		// create ipv4inipv4 flow
		if len(f.innerV4Dst) > 0 {
			dInV4.withInnerV6 = false
			dInV4.innerV4DstStart = f.innerV4Dst[0]
			dInV4.innerFlowCount = uint32(len(f.innerV4Dst))
			dInV4.innerV4SrcStart = otgSrc2.IPv4
			dInV4.innerSrcCount = uint32(1)
			dInV4.innerDscp = f.dscp
			flows = append(flows, dInV4.createTrafficFlow(fmt.Sprintf("b%d4in4%s%d:%s", batch, name, j, dscpToString(f.dscp)), f.dscp))
			fmt.Printf("b%d4in4%s%d:%s, outerFlowCount %d, innerFlowCount %d\n", batch, name, j, dscpToString(f.dscp), dInV4.outerFlowCount, dInV4.innerFlowCount)

		}
		// create ipv6inipv4 flow
		if len(f.innerV6Dst) > 0 {
			dInV4.withInnerV6 = true
			dInV4.innerV6DstStart = f.innerV6Dst[0] //example encapVrfAIPv6Enries for a batch
			dInV4.innerFlowCount = uint32(len(f.innerV6Dst))
			dInV4.innerV6SrcStart = otgSrc2.IPv6
			dInV4.innerSrcCount = uint32(1)
			dInV4.innerDscp = f.dscp
			flows = append(flows, dInV4.createTrafficFlow(fmt.Sprintf("b%d6in4%s%d:%s", batch, name, j+1, dscpToString(f.dscp)), f.dscp))
			fmt.Printf("b%d6in4%s%d:%s, outerFlowCount %d, innerFlowCount %d\n", batch, name, j+1, dscpToString(f.dscp), dInV4.outerFlowCount, dInV4.innerFlowCount)
		}
	}
	return flows

}

// getDecapFlowsForBatch creates decap flows for a given batch
func getDecapFlowsForBatch(batch int, name string, dfa ...*DecapFlowAttr) []gosnappi.Flow {

	var dInV4 = trafficflowAttr{
		withInnerHeader: true, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName2 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc2.MAC,                     // source MAC address
		dstMac:          dutSrc2.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}

	flows := []gosnappi.Flow{}

	for i, f := range dfa {
		j := i * 2
		// create ipv4inipv4 flow
		if len(f.innerV4Dst) > 0 {
			dInV4.withInnerV6 = false
			dInV4.outerDst = f.outerIP
			dInV4.outerSrc = getOuterSrcForDscp(f.dscp)
			dInV4.innerDst = f.innerV4Dst //encapVrfAIPv4Enries
			dInV4.innerSrc = otgSrc2.IPv4
			dInV4.innerDscp = f.dscp
			flows = append(flows, dInV4.createTrafficFlow(fmt.Sprintf("b%d4in4%s%d:%s", batch, name, j, dscpToString(f.dscp)), f.dscp))
			fmt.Printf("b%d4in4%s%d:%s, outerFlowCount %d, innerFlowCount %d\n", batch, name, j, dscpToString(f.dscp), len(f.outerIP), len(f.innerV4Dst))
		}
		// create ipv6inipv4 flow
		if len(f.innerV6Dst) > 0 {
			dInV4.withInnerV6 = true
			dInV4.outerSrc = getOuterSrcForDscp(f.dscp)
			dInV4.innerDst = f.innerV6Dst //encapVrfAIPv6Enries
			dInV4.innerSrc = otgSrc2.IPv6
			dInV4.innerDscp = f.dscp
			flows = append(flows, dInV4.createTrafficFlow(fmt.Sprintf("b%d6in4%s%d:%s", batch, name, j+1, dscpToString(f.dscp)), f.dscp))
			fmt.Printf("b%d6in4%s%d:%s, outerFlowCount %d, innerFlowCount %d\n", batch, name, j+1, dscpToString(f.dscp), len(f.outerIP), len(f.innerV6Dst))
		}
	}
	return flows

}

// getEncapFlowsForBatch creates encap flows for a given batch
func getEncapFlowsForBatch(batch int, name string, efa ...*EncapFlowAttr) []gosnappi.Flow {

	// encap flow attribute
	var enFa = trafficflowAttr{
		withInnerHeader: false, // flow type
		withNativeV6:    false,
		withInnerV6:     false,
		outerSrc:        v4DefaultSrc,                    // source IP address
		outerDst:        []string{v4BGPDefaultStart},     // destination IP address
		srcPort:         []string{lagName1 + ".IPv4"},    // source OTG port
		dstPorts:        []string{otgDst.Name + ".IPv4"}, // destination OTG ports
		srcMac:          otgSrc1.MAC,                     // source MAC address
		dstMac:          dutSrc1.MAC,                     // destination MAC address
		topo:            gosnappi.NewConfig(),
	}

	flows := []gosnappi.Flow{}

	for i, f := range efa {
		j := i * 2
		if len(f.outerV4Dst) > 0 {
			enFa.withNativeV6 = false
			enFa.srcPort = []string{lagName1 + ".IPv4"}
			enFa.outerSrc = v4DefaultSrc
			enFa.outerDst = f.outerV4Dst
			flows = append(flows, enFa.createTrafficFlow(fmt.Sprintf("b%dipv4%s%d:%s", batch, name, j, dscpToString(f.dscp)), f.dscp))

		}

		if len(f.outerV6Dst) > 0 {
			enFa.withNativeV6 = true
			enFa.srcPort = []string{lagName1 + ".IPv6"}
			enFa.outerSrc = innerSrcIPv6Start
			enFa.outerDst = f.outerV6Dst
			flows = append(flows, enFa.createTrafficFlow(fmt.Sprintf("b%dipv6%s%d:%s", batch, name, j, dscpToString(f.dscp)), f.dscp))
		}
	}

	return flows
}

// isCIDR checks if the input string is a valid CIDR (e.g., "192.168.0.0/24").
func isCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

func EnsureCIDR(ipStr string, mask int) string {
	// If already contains '/', we assume it's a CIDR
	if strings.Contains(ipStr, "/") {
		return ipStr
	} else {
		// Otherwise, add the mask
		return fmt.Sprintf("%s/%d", ipStr, mask)
	}
}

func GetFibSegmentGribiEntries(t *testing.T, routeParams *routesParam, dut *ondatra.DUTDevice, batchCount int) []PairedEntries {
	var pairedEntries []PairedEntries

	// Calculate the batch size dynamically based on the total number of ipEntries and batchCount
	totalEntries := len(routeParams.ipEntries)
	batchSize := (totalEntries + batchCount - 1) / batchCount // Round up to ensure all entries are included

	// Calculate the batch-specific ranges for nextHops
	nextHopsPerBatch := len(routeParams.nextHops) / batchCount

	// If nextHops are fewer than the batch size, allow all batches to reuse the same nextHops
	if len(routeParams.nextHops) < batchSize {
		t.Logf("%s: NextHops are fewer than the batch size (%d < %d), reusing the same nextHop prefixes for all batches", routeParams.segment, len(routeParams.nextHops), batchSize)
		nextHopsPerBatch = len(routeParams.nextHops)
	}
	// undo if needed
	// if routeParams.numUniqueNHs == 0 {
	// 	t.Logf("numUniqueNHs is not set, calculating it based on numUniqueNHGs and numNHPerNHG")
	// 	routeParams.numUniqueNHs = routeParams.numUniqueNHGs * routeParams.numNHPerNHG
	// }

	// avoid divide by zero and ensure that each batch gets at least one NHG
	// routeParams.numUniqueNHGs should be greater than batchCount or equal to it
	if routeParams.numUniqueNHGs < batchCount {
		t.Logf("%s: numUniqueNHGs is less than batchCount (%d < %d), setting numUniqueNHGs to batchCount", routeParams.segment, routeParams.numUniqueNHGs, batchCount)
		routeParams.numUniqueNHGs = batchCount
	}
	t.Logf("%s: numUniqueNHGs: %d, numNHPerNHG: %d, nextHopsPerBatch: %d, batchSize: %d, totalEntries: %d",
		routeParams.segment, routeParams.numUniqueNHGs, routeParams.numNHPerNHG, nextHopsPerBatch, batchSize, totalEntries)

	// retain for debugging in future
	// var NHGIDs []uint64
	// var NHIDs []uint64
	// var prefixes []string
	// var indexPrefixLimitNHGID []uint64
	for batch := 0; batch < batchCount; batch++ {
		startIndex := batch * batchSize
		endIndex := startIndex + batchSize
		if endIndex > totalEntries {
			endIndex = totalEntries
		}

		// Create a new PairedEntries for this batch
		pe := PairedEntries{}

		// Calculate the batch-specific nextHops
		var batchNextHops []string
		if len(routeParams.nextHops) < batchSize {
			batchNextHops = routeParams.nextHops // Reuse the same nextHops for all batches
		} else {
			batchNextHops = routeParams.nextHops[batch*nextHopsPerBatch : (batch+1)*nextHopsPerBatch]
		}

		numNHGsPerBatch := routeParams.numUniqueNHGs / batchCount

		// Calculate how many prefixes each NHG should get
		batchEntries := endIndex - startIndex
		ceilingRatio := int(math.Ceil(float64(batchEntries) / float64(numNHGsPerBatch)))
		floorRatio := int(math.Floor(float64(batchEntries) / float64(numNHGsPerBatch)))

		// Compute how many NHGs use ceiling and floor ratios
		useCeilingCount := batchEntries - (floorRatio * numNHGsPerBatch)
		useFloorCount := numNHGsPerBatch - useCeilingCount
		t.Logf("%s: batchEntries: %d, numNHGsPerBatch: %d, ceilingRatio: %d, floorRatio: %d, useCeilingCount: %d, useFloorCount: %d",
			routeParams.segment, batchEntries, numNHGsPerBatch, ceilingRatio, floorRatio, useCeilingCount, useFloorCount)

		// Generate NHG IDs for this batch
		nhgIDs := make([]uint64, numNHGsPerBatch)
		for i := range nhgIDs {
			nhgIDs[i] = GlobalIDPool.NextNHGID()
		}

		var nhgID uint64
		// Assign prefixes to NHGs based on ceiling & floor ratios
		nhgIndex := 0
		assignedPrefixes := 0

		prefixLimit := ceilingRatio // Start with ceiling ratio for first NHGs
		for i, ip := range routeParams.ipEntries[startIndex:endIndex] {
			if i == 0 || (prefixLimit > 0 && assignedPrefixes == prefixLimit) {
				if nhgIndex >= useCeilingCount {
					prefixLimit = floorRatio // Switch to floor ratio for remaining NHGs
				}
				nhgID = nhgIDs[nhgIndex%numNHGsPerBatch]
				// indexPrefixLimitNHGID = append(indexPrefixLimitNHGID, nhgID, uint64(i), uint64(prefixLimit), uint64(nhgIndex)) // retain for debugging in future
				nhgIndex++
				assignedPrefixes = 0 // Reset the count for the next NHG
				// Generate NextHopGroup entry
				nhgEntry := fluent.NextHopGroupEntry().
					WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
					WithID(nhgID)
				if routeParams.backupNHG != 0 {
					nhgEntry.WithBackupNHG(uint64(routeParams.backupNHG))
				}

				// Generate NextHop entries and add them to the NextHopGroup
				for j := 0; j < routeParams.numNHPerNHG; j++ {
					nhID := GlobalIDPool.NextNHID()
					nhEntry := fluent.NextHopEntry().
						WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(nhID).
						WithNextHopNetworkInstance(routeParams.nextHopVRF)
					if routeParams.nextHopType == "encap" {
						nhEntry.WithIPinIP(routeParams.tunnelSrcIP, batchNextHops[((nhgIndex-1)*routeParams.numNHPerNHG+j)%len(batchNextHops)]).
							WithEncapsulateHeader(fluent.IPinIP)
					} else if routeParams.nextHopType == "decap" {
						nhEntry.WithDecapsulateHeader(fluent.IPinIP)
					} else if routeParams.nextHopType == "decapEncap" {
						nhEntry.WithDecapsulateHeader(fluent.IPinIP).
							WithEncapsulateHeader(fluent.IPinIP).
							WithIPinIP(routeParams.tunnelSrcIP, batchNextHops[((nhgIndex-1)*routeParams.numNHPerNHG+j)%len(batchNextHops)])
					} else if routeParams.nextHopType == "default" {
						nhEntry.WithIPAddress(batchNextHops[((nhgIndex-1)*routeParams.numNHPerNHG+j)%len(batchNextHops)])
					}
					pe.NHs = append(pe.NHs, nhEntry)
					// NHIDs = append(NHIDs, nhID) // retain for debugging in future
					// Add the NextHop to the NextHopGroup
					nhgEntry.AddNextHop(nhID, uint64(routeParams.nextHopWeight[j]))
				}
				// NHGIDs = append(NHGIDs, nhgID)
				pe.NHGs = append(pe.NHGs, nhgEntry)
			}
			assignedPrefixes++
			// Generate IPv4 entry
			ipCIDR := ip // for variable length prefix
			if !isCIDR(ip) {
				ipCIDR = EnsureCIDR(ip, 32) // Ensure that the IPv4 address has a mask length of 32
				pe.V4Prefixes = append(pe.V4Prefixes, ip)
			} else { // for variable length prefix
				pe.V4Prefixes = append(pe.V4Prefixes, iputil.GenerateIPs(ip, routeParams.addrPerSubnet)...)
			}

			ipv4Entry := fluent.IPv4Entry().
				WithPrefix(ipCIDR).
				WithNetworkInstance(routeParams.prefixVRF).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
			pe.V4Entries = append(pe.V4Entries, ipv4Entry)

			// Generate IPv6 entries for this batch
			if len(routeParams.ipv6Entries) > 0 {
				ip = routeParams.ipv6Entries[startIndex:endIndex][i]
				ipCIDR := EnsureCIDR(ip, 128) // Ensure that the IPv6 address has a mask length of 128
				ipv6Entry := fluent.IPv6Entry().
					WithPrefix(ipCIDR).
					WithNetworkInstance(routeParams.prefixVRF).
					WithNextHopGroup(nhgID).
					WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
				pe.V6Entries = append(pe.V6Entries, ipv6Entry)
				pe.V6Prefixes = append(pe.V6Prefixes, ip)
			}
			// prefixes = append(prefixes, ip) // retain for debugging in future
		}

		// Add the PairedEntries for this batch to the result
		pairedEntries = append(pairedEntries, pe)
		t.Logf("%s: Batch %d: NHGs: %dn, NHs: %dn, IPv4 entries: %d, IPv6 entries: %d, Prefixes: %d",
			routeParams.segment, batch, len(pe.NHGs), len(pe.NHs), len(pe.V4Entries), len(pe.V6Entries), len(pe.V4Prefixes))
		// retain for debugging in future
		// t.Logf("%s: Batch %d: NHGIDs: %v\n, NHIDs: %v\n, Prefixes: %v\n",
		// 	routeParams.segment, batch, NHGIDs, NHIDs, prefixes)
		// t.Logf("%s: Batch %d: indexPrefixLimitNHGID: %v\n",
		// 	routeParams.segment, batch, indexPrefixLimitNHGID)
		// NHGIDs = nil
		// NHIDs = nil
		// prefixes = nil
		// indexPrefixLimitNHGID = nil
	}

	return pairedEntries
}

func LogGribiInfo(t *testing.T, segment string, gribiInfo []PairedEntries) {
	for i, pe := range gribiInfo {
		t.Logf("Segment %s, Batch %d: IPv4 entries: %d, IPv6 entries: %d, NH entries: %d, NHG entries: %d",
			segment, i, len(pe.V4Entries), len(pe.V6Entries), len(pe.NHs), len(pe.NHGs))

		for j, nhg := range pe.NHGs {
			t.Logf("Batch %d, NHG %d: %v", i, j, nhg)
		}
		for j, nh := range pe.NHs {
			t.Logf("Batch %d, NH %d: %v", i, j, nh)
		}
		for j, ipv4 := range pe.V4Entries {
			t.Logf("Batch %d, IPv4 %d: %v", i, j, ipv4)
		}
		for j, ipv6 := range pe.V6Entries {
			t.Logf("Batch %d, IPv6 %d: %v", i, j, ipv6)
		}
		for j, ipv4 := range pe.V4Prefixes {
			t.Logf("Batch %d, IPv4 Prefix %d: %v", i, j, ipv4)
		}
		for j, ipv6 := range pe.V6Prefixes {
			t.Logf("Batch %d, IPv6 Prefix %d: %v", i, j, ipv6)
		}
	}
}

func getVariableLenSubnets(subNets uint32, seedBlocks ...string) []string {
	variableLenSubnets := []string{}
	for _, seedBlock := range seedBlocks {
		variableLenSubnets = append(variableLenSubnets, generateIPv4Subnets(seedBlock, subNets)...)
	}
	return variableLenSubnets
}

type ResourceData struct {
	AvailableResourceIDs int
	ClientUsages         map[string]int // Map indexed by client names
}

func getGridPoolUsage(input string) ResourceData {
	var data ResourceData
	data.ClientUsages = make(map[string]int) // Initialize the map

	// Split the input into lines
	lines := strings.Split(input, "\n")

	// Regular expressions to match available resource IDs, client, and usage
	availableRegex := regexp.MustCompile(`(?i)Available resource IDs\s+:\s+(\d+)`)
	clientRegex := regexp.MustCompile(`(?i)Client\s+:\s+(\S+)`)
	usageRegex := regexp.MustCompile(`(?i)current usage\s+:\s+(\d+)`)

	var currentClient string

	for _, line := range lines {
		// Check for available resource IDs
		if availableMatch := availableRegex.FindStringSubmatch(line); availableMatch != nil {
			available, err := strconv.Atoi(availableMatch[1])
			if err == nil {
				data.AvailableResourceIDs = available
			}
		}

		// Check for client line
		if clientMatch := clientRegex.FindStringSubmatch(line); clientMatch != nil {
			currentClient = clientMatch[1]
		}

		// Check for usage line
		if usageMatch := usageRegex.FindStringSubmatch(line); usageMatch != nil && currentClient != "" {
			usage, err := strconv.Atoi(usageMatch[1])
			if err == nil {
				data.ClientUsages[currentClient] = usage // Add to the map
				currentClient = ""                       // Reset for next client
			}
		}
	}

	return data
}

// getGridPoolUsage parses the output of the "show grid pool" command and returns
// the available resource IDs and the current usage for each client.
func getGridPoolUsageViaGNMI(t *testing.T, dut *ondatra.DUTDevice, pool, bank int, location string) ResourceData {
	input := util.SshRunCommand(t, dut, fmt.Sprintf("show grid pool %d bank %d location %s", pool, bank, location))
	t.Log("command output using ssh\n", input)
	// todo: use gnmi to get the output instead of ssh
	// input := util.CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("show grid pool %d bank %d location %s", pool, bank, location))
	// t.Log("command output using gnmi\n", input)
	return getGridPoolUsage(input)
}

func configureBaseInfra(t *testing.T, bc *baseConfig) *testArgs {
	dut := ondatra.DUT(t, "dut")
	peer := ondatra.DUT(t, "peer")
	otg := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	t.Log("Configure DUT & PEER devices")
	configureDevices(t, dut, peer, "bundle")
	t.Log("Configure TGEN OTG")
	topo := configureOTG(t, otg, dut, peer)
	t.Log("OTG CONFIG: ", topo)
	tcArgs := initializeTestResources(t, dut, peer, otg, topo, client, ctx)
	initTriggers(tcArgs)

	t.Run("Verify default BGP traffic", func(t *testing.T) {
		v4BGPFlow := defaultV4.createTrafficFlow("DefaultV4", dscpEncapNoMatch)
		validateTrafficFlows(t, tcArgs, []gosnappi.Flow{v4BGPFlow}, false, true)
	})

	// add static route on peer for the tunnel destination for encap, decap+encap traffic
	configStaticRoute(t, peer, "200.200.0.0/16", otgDst.IPv4, "", "", false)
	configStaticRoute(t, peer, "100.101.0.0/16", otgDst.IPv4, "", "", false)
	gArgs = tcArgs
	bConfig.setConfigured(true)
	return tcArgs
}

func initializeTestResources(t *testing.T, dut, peer *ondatra.DUTDevice, otg *ondatra.ATEDevice, topo gosnappi.Config, client *fluent.GRIBIClient, ctx context.Context) *testArgs {
	// once.Do(func() {
	t.Helper() // Mark this function as a test helper
	tcArgs := &testArgs{
		dut:    dut,
		peer:   peer,
		ate:    otg,
		topo:   topo,
		client: client,
		ctx:    ctx,
	}

	tcArgs.LogDir = *logDir

	// t.Log("Get List of IPs on NH PEER for DUT-Peer Bundle interfaces")
	tcArgs.primaryPaths = pathInfo.PrimaryPathsPeerV4
	tcArgs.primaryPaths = append(tcArgs.primaryPaths, pathInfo.PrimarySubintfPathsV4...)
	tcArgs.frr1Paths = pathInfo.BackupPathsPeerV4
	tcArgs.frr1Paths = append(tcArgs.frr1Paths, pathInfo.BackupSubintfPathsV4...)
	// get and update available LCs
	tcArgs.DUT = DUTResources{
		Device: dut,
		GNMI:   dut.RawAPIs().GNMI(t),
		GNSI:   dut.RawAPIs().GNSI(t),
		// GNPSI:       dut.RawAPIs().GNPSI(t),
		CLI:  dut.RawAPIs().CLI(t),
		P4RT: dut.RawAPIs().P4RT(t),
		// Console:     dut.RawAPIs().Console(t),
		OSC:         dut.RawAPIs().GNOI(t).OS(),
		SC:          dut.RawAPIs().GNOI(t).System(),
		GRIBI:       dut.RawAPIs().GRIBI(t),
		FluentGRIBI: fluent.NewClient(),
		LCs:         util.GetLCList(t, dut),
		ActiveRP:    "",
		StandbyRP:   "",
		// updated later using method calls
		// DualSup:     false,
		// ActiveRP:    "",
		// StandbyRP:   "",
	}
	tcArgs.PEER = DUTResources{
		Device: peer,
		GNMI:   peer.RawAPIs().GNMI(t),
		GNSI:   peer.RawAPIs().GNSI(t),
		// GNPSI:       peer.RawAPIs().GNPSI(t),
		CLI:  peer.RawAPIs().CLI(t),
		P4RT: peer.RawAPIs().P4RT(t),
		// Console:     peer.RawAPIs().Console(t),
		OSC:         peer.RawAPIs().GNOI(t).OS(),
		SC:          peer.RawAPIs().GNOI(t).System(),
		GRIBI:       peer.RawAPIs().GRIBI(t),
		FluentGRIBI: fluent.NewClient(),
		LCs:         util.GetLCList(t, peer),
		// updated later using method calls
		// DualSup:     false,
		// ActiveRP:    "",
		// StandbyRP:   "",
	}
	tcArgs.OTG = OTGResources{
		Device: otg,
		GNMI:   otg.RawAPIs().GNMI(t),
	}
	// Detect Dual sup support for DUT
	dutDualSup, err := hautils.HasDualSUP(tcArgs.ctx, tcArgs.DUT.OSC)
	if err != nil {
		t.Logf("fetching dual sup info failed, Error:%v", err)
	}
	tcArgs.DUT.DualSup = dutDualSup
	if tcArgs.DUT.DualSup {
		tcArgs.DUT.StandbyRP, tcArgs.DUT.ActiveRP = components.FindStandbyControllerCard(t, dut, components.FindComponentsByType(t, dut, controlcardType))
	} else {
		tcArgs.DUT.ActiveRP = FindActiveControllerCard(t, dut, components.FindComponentsByType(t, dut, controlcardType))
	}
	// Detect Dual sup support for PEER
	peerDualSup, err := hautils.HasDualSUP(tcArgs.ctx, tcArgs.PEER.OSC)
	if err != nil {
		t.Logf("fetching dual sup info failed, Error:%v", err)
	}
	tcArgs.PEER.DualSup = peerDualSup
	if tcArgs.PEER.DualSup {
		tcArgs.PEER.StandbyRP, tcArgs.PEER.ActiveRP = components.FindStandbyControllerCard(t, dut, components.FindComponentsByType(t, dut, controlcardType))
	} else {
		tcArgs.PEER.ActiveRP = FindActiveControllerCard(t, dut, components.FindComponentsByType(t, dut, controlcardType))
	}

	// Start fluent connection
	tcArgs.DUT.FluentGRIBI.Connection().WithStub(tcArgs.DUT.GRIBI).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()
	tcArgs.DUT.FluentGRIBI.Start(tcArgs.ctx, t)

	// start log collection
	log_collector.Start(tcArgs.ctx, t, tcArgs.DUT.Device)

	tcArgs.CommandPatterns = getCommandPatterns(t)

	// t.Log("Get List of IPs on NH PEER for DUT-Peer Bundle interfaces")
	tcArgs.primaryPaths = pathInfo.PrimaryPathsPeerV4
	tcArgs.primaryPaths = append(tcArgs.primaryPaths, pathInfo.PrimarySubintfPathsV4...)
	tcArgs.frr1Paths = pathInfo.BackupPathsPeerV4
	tcArgs.frr1Paths = append(tcArgs.frr1Paths, pathInfo.BackupSubintfPathsV4...)
	// })
	return tcArgs
}

func initTriggers(tcArgs *testArgs) {

	processes := []string{
		"bgp",
		"ifmgr",
		"db_writer",
		"isis",
	}

	triggers = []Trigger{
		{
			name: "RPFO",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.Dorpfo(ctx, t, false)
			},
			duration:              1 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       true,
		},
		{
			name: "LC-Reboot",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.DoAllAvailableLcParallelReboot(t, tcArgs.DUT.Device)
			},
			duration:              5 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       false,
		},
		{
			name: "ProcessRestartParllel",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.DoProcessesRestart(ctx, t, tcArgs.DUT.Device, processes, true)
			},
			duration:              5 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       false,
		},
		{
			name: "ProcessRestartSequential",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.DoProcessesRestart(ctx, t, tcArgs.DUT.Device, processes, false)
			},
			duration:              5 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       false,
		},
		{
			name: "gNOI-REBOOT",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.GnoiReboot(t, tcArgs.DUT.Device)
			},
			duration:              10 * time.Minute,
			reprogrammingRequired: true,
			reconnectClient:       true,
		},
		{
			name: "LC-Shut-Unshut",
			fn: func(ctx context.Context, t *testing.T) {
				hautils.DoShutUnshutAllAvailableLcParallel(t, tcArgs.DUT.Device)
			},
			duration:              5 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       false,
		},
	}
}

func getCommandPatterns(t *testing.T) map[string]map[string]interface{} {
	if *debugCommandYaml == "" {
		// Get the current working directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current working directory: %v", err)
		}

		// Get the absolute path of the test file
		absPath, err := filepath.Abs(currentDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}
		*debugCommandYaml = absPath + "/debug.yaml"
	}

	commandPatterns, err := log_collector.ParseYAML(*debugCommandYaml)
	if err != nil {
		t.Fatalf("Debug yaml parsing failed: Error : %v", err)
	}
	return commandPatterns
}

// FindActiveControllerCard gets a list of two components and finds out the active and standby controller_cards.
func FindActiveControllerCard(t *testing.T, dut *ondatra.DUTDevice, supervisors []string) string {
	var activeCC, standbyCC string
	for _, supervisor := range supervisors {
		watch := gnmi.Watch(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State(), 10*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
			return val.IsPresent()
		})
		if val, ok := watch.Await(t); !ok {
			t.Fatalf("DUT did not reach target state within %v: got %v", 10*time.Minute, val)
		}
		role := gnmi.Get(t, dut, gnmi.OC().Component(supervisor).RedundantRole().State())
		t.Logf("Component(supervisor).RedundantRole().Get(t): %v, Role: %v", supervisor, role)
		if role == standbyController {
			standbyCC = supervisor
		} else if role == activeController {
			activeCC = supervisor
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", supervisor, role)
		}
	}
	if activeCC == "" {
		t.Fatalf("Expected non-empty activeCC and standbyCC, got activeCC: %v, standbyCC: %v", activeCC, standbyCC)
	}
	t.Logf("Detected activeCC: %v, standbyCC: %v", activeCC, standbyCC)

	return activeCC
}

// DivideAndAdjust returns:
// 1. The largest integer ≤ (x / y) that is divisible by batchCount
// 2. The updated remainder: original remainder + (x/y - adjusted quotient)
func DivideAndAdjust(resourceIDs, nhsPerNHG, batchCount int) (int, int, error) {
	if nhsPerNHG == 0 {
		return 0, 0, fmt.Errorf("division by zero")
	}

	quotient := resourceIDs / nhsPerNHG
	remainder := resourceIDs % nhsPerNHG

	// Adjust quotient down to nearest multiple of 4
	adjustedQuotient := quotient - (quotient % batchCount)

	// Calculate how much was subtracted from original quotient
	leftoverCount := quotient - adjustedQuotient

	// Add leftover count to remainder
	updatedRemainder := remainder + (leftoverCount * nhsPerNHG)

	return adjustedQuotient, updatedRemainder, nil
}

func testExpandedModularChain(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	//modular configuration begins below
	batches := 4

	// prepare backup NHG fluent Entries
	// backup used in FRR2 case
	backUpFluentEntries := []fluent.GRIBIEntry{}
	nhID := GlobalIDPool.NextNHID()
	nhgDecapToDefault := GlobalIDPool.NextNHGID()
	backUpFluentEntries = append(backUpFluentEntries,
		fluent.NextHopEntry().WithIndex(nhID).WithDecapsulateHeader(fluent.IPinIP).WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)),
	)
	backUpFluentEntries = append(backUpFluentEntries,
		fluent.NextHopGroupEntry().WithID(nhgDecapToDefault).AddNextHop(nhID, 1).WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)),
	)

	// backup used in FRR1 case
	nhgRedirectToVrfR := GlobalIDPool.NextNHGID()
	nhID = GlobalIDPool.NextNHID()
	// build backup NHG and NH.
	backUpFluentEntries = append(backUpFluentEntries,
		fluent.NextHopEntry().WithIndex(nhID).WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).WithNextHopNetworkInstance(VRFR),
	)
	backUpFluentEntries = append(backUpFluentEntries,
		fluent.NextHopGroupEntry().WithID(nhgRedirectToVrfR).AddNextHop(nhID, 1).WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)),
	)

	t.Logf("Peer NH IP: %v", tcArgs.primaryPaths)
	level1Primary := routesParam{
		ipEntries:     vipIPs, // 512 VIP prefixes
		prefixVRF:     deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHops:      tcArgs.primaryPaths, // peer or otg prefixes, peerNHIP, _ := getDUTBundleIPAddrList(peerBundleIPMap)
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "default",
		numUniqueNHGs: L1Nhg,      // 512
		numNHPerNHG:   L1NhPerNHG, //8
		nextHopWeight: generateNextHopWeights(64, 8),
	}

	gribiInfo := GetFibSegmentGribiEntries(t, &level1Primary, tcArgs.dut, batches)
	LogGribiInfo(t, "level1Primary", gribiInfo)

	level2Primary := routesParam{
		ipEntries:     tunnelDestIPs, // 1600 tunnel prefixes - will be 6400 in final
		prefixVRF:     vrfTransit,
		nextHops:      vipIPs, // VIP addresses
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "default",
		numUniqueNHGs: L2Nhg, // 256
		numNHPerNHG:   2,     // each prefix uses a NHG with 2 NHs
		nextHopWeight: generateNextHopWeights(256, L2NhPerNHG),
		backupNHG:     int(nhgRedirectToVrfR),
	}

	gribiInfo = GetFibSegmentGribiEntries(t, &level2Primary, tcArgs.dut, batches)
	LogGribiInfo(t, "level2Primary", gribiInfo)

	level3PrimaryA := routesParam{
		ipEntries:     encapVrfAIPv4Enries,
		ipv6Entries:   encapVrfAIPv6Enries,
		prefixVRF:     vrfEncapA,
		nextHops:      tunnelDestIPs,
		nextHopVRF:    vrfTransit,
		nextHopType:   "encap",
		startNHIndex:  lastNhIndex + 1, // not used
		numUniqueNHGs: 200,             //encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	encapEntriesA := GetFibSegmentGribiEntries(t, &level3PrimaryA, tcArgs.dut, batches)
	LogGribiInfo(t, "level3PrimaryA", encapEntriesA)

	level3PrimaryB := routesParam{
		ipEntries:     encapVrfBIPv4Enries,
		ipv6Entries:   encapVrfBIPv6Enries,
		prefixVRF:     vrfEncapB,
		nextHops:      tunnelDestIPs,
		nextHopVRF:    vrfTransit,
		nextHopType:   "encap",
		numUniqueNHGs: 200, //encapNhgcount,
		numNHPerNHG:   8,
		nextHopWeight: generateNextHopWeights(16, 8),
		tunnelSrcIP:   ipv4OuterSrc111,
	}

	encapEntriesB := GetFibSegmentGribiEntries(t, &level3PrimaryB, tcArgs.dut, batches)

	level1Frr1 := routesParam{
		ipEntries:     vipFrr1IPs, // 512 VIP prefixes
		prefixVRF:     deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHops:      tcArgs.primaryPaths, // peer or otg prefixes
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "default",
		numUniqueNHGs: L1Nhg,      // 512
		numNHPerNHG:   L1NhPerNHG, //8
		nextHopWeight: generateNextHopWeights(64, 8),
	}

	gribiInfo = GetFibSegmentGribiEntries(t, &level1Frr1, tcArgs.dut, batches)
	LogGribiInfo(t, "level1Frr1", gribiInfo)

	level2Frr1 := routesParam{
		ipEntries:     tunnelDestIPs, // 1600 tunnel prefixes - will be 6800 in final
		prefixVRF:     VRFR,
		nextHops:      vipFrr1IPs, // VIP addresses. Tunnel Dest IPs are same as VIPs
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "decapEncap",
		numUniqueNHGs: L2Nhg,      // 256
		numNHPerNHG:   L2NhPerNHG, // 8
		nextHopWeight: generateNextHopWeights(256, L2NhPerNHG),
		backupNHG:     int(nhgDecapToDefault),
		tunnelSrcIP:   ipv4OuterSrc222,
	}

	gribiInfo = GetFibSegmentGribiEntries(t, &level2Frr1, tcArgs.dut, batches)
	LogGribiInfo(t, "level2Frr1", gribiInfo)

	decapWan := routesParam{
		ipEntries:     iputil.GenerateIPs(IPBlockDecap, decapIPv4ScaleCount),
		prefixVRF:     niDecapTeVrf,
		nextHops:      []string{}, // not used for decap
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "decap",
		numUniqueNHGs: 1000, //encapNhgcount,
		numNHPerNHG:   1,
		nextHopWeight: generateNextHopWeights(1, 1),
	}

	decapWanPE := GetFibSegmentGribiEntries(t, &decapWan, tcArgs.dut, batches)
	LogGribiInfo(t, "decapWan", decapWanPE)

	decapWanVarPrefix := routesParam{
		ipEntries:     getVariableLenSubnets(12, "102.51.100.1/22", "107.51.105.1/24", "112.51.110.1/26", "117.51.115.1/28"),
		addrPerSubnet: 1,
		prefixVRF:     niDecapTeVrf,
		nextHops:      []string{}, // not used for decap
		nextHopVRF:    deviations.DefaultNetworkInstance(tcArgs.dut),
		nextHopType:   "decap",
		numUniqueNHGs: 48,
		numNHPerNHG:   1,
		nextHopWeight: generateNextHopWeights(1, 1),
	}

	decapWanVp := GetFibSegmentGribiEntries(t, &decapWanVarPrefix, tcArgs.dut, batches)
	LogGribiInfo(t, "decapWanVp", decapWanVp)

	configBatches := CombinePairedEntries(t, tcArgs.dut, batches, &level1Primary, &level2Primary, &level3PrimaryA, &level3PrimaryB, &level1Frr1, &level2Frr1, &decapWan, &decapWanVarPrefix)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	// Program backup entries first
	t.Logf("Programming backup entries")
	tcArgs.client.Modify().AddEntry(t, backUpFluentEntries...)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, 1*time.Minute); err != nil {
		t.Fatalf("Could not program entries, got err: %v", err)
	}

	// only program the first batch
	entries := configBatches[0]

	t.Logf("Programming %d entries", len(entries))
	tcArgs.client.Modify().AddEntry(t, entries...)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, aftProgTimeout); err != nil {
		t.Fatalf("Could not program entries, got err: %v", err)
	}

	t.Logf("Validating encap traffic")
	validateTrafficFlows(t, tcArgs, getEncapFlowsForBatch(0, "encpA", &EncapFlowAttr{encapEntriesA[0].V4Prefixes, encapEntriesA[0].V6Prefixes, dscpEncapA1}), false, true)

	t.Logf("Validating variable length prefix decap traffic")
	validateTrafficFlows(t, tcArgs, getDecapFlowsForBatch(0, "dcapV",
		&DecapFlowAttr{decapWanVp[0].V4Prefixes, encapEntriesA[0].V4Prefixes, encapEntriesA[0].V6Prefixes, dscpEncapA1},
		&DecapFlowAttr{decapWanVp[0].V4Prefixes, encapEntriesB[0].V4Prefixes, encapEntriesB[0].V6Prefixes, dscpEncapB1}),
		false, true)

	t.Logf("Validating fixed length prefix decap traffic")
	validateTrafficFlows(t, tcArgs, getDecapFlowsForBatch(0, "dcapF",
		&DecapFlowAttr{decapWanPE[0].V4Prefixes, encapEntriesA[0].V4Prefixes, encapEntriesA[0].V6Prefixes, dscpEncapA1},
		&DecapFlowAttr{decapWanPE[0].V4Prefixes, encapEntriesB[0].V4Prefixes, encapEntriesB[0].V6Prefixes, dscpEncapB1}),
		false, true)
}

func testCompactModularChain(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	//modular configuration begins below

	// batches := 1
	// single encap tunnel case, 3 decap (1 fixed, 1 variable, one frr2backup), 1 decapEncap (frr1backup) cases
	// gp := NewGribiProfile(t, batches, true, true, dut,
	// 	&routesParam{segment: "PrimaryLevel1", nextHops: peerNHIP, numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "PrimaryLevel2", numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "Frr1Level1", nextHops: peerNHIP, numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "Frr1Level2", numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "DecapWan", numUniqueNHGs: 1, numNHPerNHG: 1},
	// 	&routesParam{segment: "DecapWanVar", numUniqueNHGs: 1, numNHPerNHG: 1},
	// )

	batches := 2
	// case 100*8*2vrf=1600 encap, 1 decap, 1 decapEncap, 1 frr2backup, 1 frr1backup
	gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths, numUniqueNHGs: 2, numNHPerNHG: 1}, //primary path
		&routesParam{segment: "PrimaryLevel2", numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: 100, numNHPerNHG: 8},
		&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 100, numNHPerNHG: 8},
		&routesParam{segment: "Frr1Level1", nextHops: tcArgs.frr1Paths, numUniqueNHGs: 1, numNHPerNHG: 1}, //frr1 path
		&routesParam{segment: "Frr1Level2", numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "DecapWan", numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "DecapWanVar", numUniqueNHGs: 2, numNHPerNHG: 1},
	)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	t.Run("Push batch 1", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{1})
	})
	t.Run("Push batch 0", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0})
	})

	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1})
	})

	t.Run("Validating variable length prefix decap traffic", func(t *testing.T) {
		testDecapTrafficFlows(t, tcArgs, gp, []int{0, 1})
	})
}

// testEncapScale tests the scale of (16k-already in use) encap next hop entries
func testEncapScale(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	// distribute the available resource IDs to NHGs such that it can be divided in batches.
	// remaining resource IDs will be used to configure using a single NHG with nhs count = nhLeftover
	var gridRsrc ResourceData
	t.Run("Get grid pool usage", func(t *testing.T) {
		gridRsrc = getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)

	})
	batches := 2
	nhsPerNHG := 8
	nhg, nhLeftover, _ := DivideAndAdjust(gridRsrc.AvailableResourceIDs, nhsPerNHG, batches)
	t.Logf("Possible NHG: %d, leftover: %d with available %d resource IDs", nhg, nhLeftover, gridRsrc.AvailableResourceIDs)

	gp := NewGribiProfile(t, batches, false, false, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths, numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG),
			numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: nhg, numNHPerNHG: nhsPerNHG, nextHopWeight: generateNextHopWeights(64, nhsPerNHG)},
	)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	// configure leftover NHGs
	t.Run("Configure leftover NHGs", func(t *testing.T) {
		if nhLeftover > 0 {
			remaingNhGp := NewGribiProfile(t, 1, false, false, tcArgs.dut,
				&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 1, numNHPerNHG: nhLeftover, nextHopWeight: generateNextHopWeights(64, nhLeftover)},
			)
			remaingNhGp.pushBatchConfig(t, tcArgs, []int{0})
		} else {
			t.Skip("no leftover NHGs to configure")
		}
	})
	t.Log("Measure performance for each PrimaryUniqueIntfCard")
	for _, card := range pathInfo.PrimaryUniqueIntfCards {
		t.Logf("Processing PrimaryUniqueIntfCard: %v", card)
		gp.measurePerf = &tunTypes{location: card, tunType: []string{"iptnlencap"}}
	}

	t.Run("Push batch config", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1})
		time.Sleep(10 * time.Second)
	})

	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1})
	})

	t.Run("Push configuration again (twice)", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1})
	})
	t.Run("Validating encap traffic again after pushing configuration twice", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1})
	})

	t.Run("Delete batch configuration", func(t *testing.T) {
		// delete the configuration
		gp.DeleteBatchConfig(t, tcArgs, []int{0, 1})
	})
}

// testDecapScale tests the scale of decap tunnel next hop entries
func testDecapScale(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	batches := 4

	// distribute the available resource IDs to NHGs such that it can be divided in batches.
	// remaining resource IDs will be used to configure using a single NHG with nhs count = nhLeftover
	var gridRsrc ResourceData
	t.Log("Get grid pool usage")
	gridRsrc = getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)

	// available resource IDs for decap = 4k- already used by other clients
	// 200 deducted in attempt to find how many entries can be configured for decap
	availableForDecap := 4096 - gridRsrc.ClientUsages["eth_intf_ma_lc"] - 200

	nhsPerNHG := 1

	// reserve resource IDs for encap that will be used for decaped traffic to pass through the encap primary path
	// reserveForEncap := batches
	// reserveForEncap := 2

	nhg, nhLeftover, _ := DivideAndAdjust(availableForDecap, nhsPerNHG, batches)
	t.Logf("Possible NHG: %d, leftover: %d with available %d resource IDs", nhg, nhLeftover, availableForDecap)

	gp := NewGribiProfile(t, batches, false, false, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths, numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, batches), //have as minimum VipIPs
			numUniqueNHGs: 2, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: batches, numNHPerNHG: 1, nextHopWeight: generateNextHopWeights(64, 1)},
		&routesParam{segment: "DecapWan", numUniqueNHGs: nhg, numNHPerNHG: nhsPerNHG, ipEntries: iputil.GenerateIPs(IPBlockDecap, nhg*nhsPerNHG), nextHopWeight: generateNextHopWeights(64, nhsPerNHG)},
	)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	t.Run("Push gribi programming batch config", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3})
	})

	t.Run("Configure leftover NHGs", func(t *testing.T) {
		// configure leftover NHGs
		if nhLeftover > 0 {
			if nhLeftover > MaxNhsPerNHG {
				// configure leftover NHGs
				t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3C", MaxNhsPerNHG)
				remaingNhGp1 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
					&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 1, numNHPerNHG: MaxNhsPerNHG, nextHopWeight: generateNextHopWeights(256, MaxNhsPerNHG)},
				)
				remaingNhGp1.pushBatchConfig(t, tcArgs, []int{0})
				//leftover NHGs after deducting MaxNhsPerNHG
				nhLeftover -= MaxNhsPerNHG
			}
			t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3B", nhLeftover)
			remaingNhGp2 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
				&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 1, numNHPerNHG: nhLeftover, nextHopWeight: generateNextHopWeights(64, nhLeftover)},
			)
			remaingNhGp2.pushBatchConfig(t, tcArgs, []int{0})
		} else {
			t.Skip("no leftover NHGs to configure")
		}
	})

	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1, 2, 3})
	})

	// testing multiple decap traffic flows per batch to avoid ixia scale issue.
	for _, batch := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("Validating Decap Traffic Flows Batch%d", batch), func(t *testing.T) {
			testDecapTrafficFlows(t, tcArgs, gp, []int{batch})
		})
	}
}

// testDecapEncapScale tests the scale of (16k-already in use) encap next hop entries
func testDecapEncapScale(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	batches := 2

	// distribute the available resource IDs to NHGs such that it can be divided in batches.
	// remaining resource IDs will be used to configure using a single NHG with nhs count = nhLeftover
	var gridRsrc ResourceData
	t.Run("Get grid pool usage", func(t *testing.T) {
		gridRsrc = getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)
	})
	// reduce resouce for 1 encap tunnel for each batch, and 2 for leftover nexthops
	availableForDecapEncap := reduceToPercent(gridRsrc.AvailableResourceIDs, UsableResoucePercent) - batches - 2
	nhsPerNHG := 8

	nhg, nhLeftover, _ := DivideAndAdjust(availableForDecapEncap, nhsPerNHG, batches)
	t.Logf("Possible NHG: %d, leftover: %d with available %d resource IDs", nhg, nhLeftover, availableForDecapEncap)

	// note route to "177.177.0.0/16" does not exist
	gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: iputil.GenerateIPs("177.177.0.0/16", len(tcArgs.primaryPaths)), numUniqueNHGs: batches, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG),
			numUniqueNHGs: batches, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: batches, numNHPerNHG: 1, nextHopWeight: generateNextHopWeights(64, 1)},
		&routesParam{segment: "Frr1Level1", nextHops: tcArgs.frr1Paths, numUniqueNHGs: 1, numNHPerNHG: 1}, //frr1 path
		&routesParam{segment: "Frr1Level2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG), numUniqueNHGs: nhg, numNHPerNHG: nhsPerNHG},
	)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	t.Run("Push gribi batch config", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1})
	})

	t.Run("Configure leftover NHGs", func(t *testing.T) {
		// configure leftover NHGs
		if nhLeftover > 0 {
			if nhLeftover > MaxNhsPerNHG {
				// configure leftover NHGs
				t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3C", MaxNhsPerNHG)
				remaingNhGp1 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
					&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 1, numNHPerNHG: MaxNhsPerNHG, nextHopWeight: generateNextHopWeights(256, MaxNhsPerNHG)},
				)
				remaingNhGp1.pushBatchConfig(t, tcArgs, []int{0})
				//leftover NHGs after deducting MaxNhsPerNHG
				nhLeftover -= MaxNhsPerNHG
			}
			t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3B", nhLeftover)
			remaingNhGp2 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
				&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 1, numNHPerNHG: nhLeftover, nextHopWeight: generateNextHopWeights(64, nhLeftover)},
			)
			remaingNhGp2.pushBatchConfig(t, tcArgs, []int{0})
		} else {
			t.Skip("no leftover NHGs to configure")
		}
	})
	t.Run("Resource consuption for all unique cards", func(t *testing.T) {
		getResouceConsumption(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, pathInfo.PrimaryUniqueIntfCards)
	})
	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1})
	})
}

// parseOfaPerf parses the output of the "show ofa performance iptnlnh location <>" command and returns
// difference between client and server timestamps.
func parseOfaPerf(cliOutput string) (time.Duration, time.Duration, error) {
	layout := "Mon Jan 2 15:04:05.000 2006"

	// Clean extra spaces and normalize lines
	cleaned := strings.ReplaceAll(cliOutput, "\t", " ")
	lines := strings.Split(cleaned, "\n")

	var clientFirstStr, clientLastStr, serverFirstStr, serverLastStr string
	reTime := regexp.MustCompile(`([A-Za-z]{3} [A-Za-z]{3} \d{2} \d{2}:\d{2}:\d{2}\.\d{3})`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "First req received:") && clientFirstStr == "":
			if match := reTime.FindStringSubmatch(line); match != nil {
				clientFirstStr = match[1]
			}
		case strings.HasPrefix(line, "Last req received:"):
			if match := reTime.FindStringSubmatch(line); match != nil {
				clientLastStr = match[1]
			}
		case strings.HasPrefix(line, "First req received:") && clientFirstStr != "":
			if match := reTime.FindStringSubmatch(line); match != nil {
				serverFirstStr = match[1]
			}
		case strings.HasPrefix(line, "Last req complete:"):
			if match := reTime.FindStringSubmatch(line); match != nil {
				serverLastStr = match[1]
			}
		}
	}

	// Validate all timestamps were found
	if clientFirstStr == "" || clientLastStr == "" || serverFirstStr == "" || serverLastStr == "" {
		return 0, 0, fmt.Errorf("missing one or more timestamps")
	}

	// Use the correct year, fallback to current year
	year := time.Now().Year()
	parseWithYear := func(tStr string) (time.Time, error) {
		return time.Parse(layout, fmt.Sprintf("%s %d", tStr, year))
	}

	clientFirst, err1 := parseWithYear(clientFirstStr)
	clientLast, err2 := parseWithYear(clientLastStr)
	serverFirst, err3 := parseWithYear(serverFirstStr)
	serverLast, err4 := parseWithYear(serverLastStr)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return 0, 0, fmt.Errorf("timestamp parse error")
	}

	return clientLast.Sub(clientFirst), serverLast.Sub(serverFirst), nil
}

func getOfaPerformance(t *testing.T, dut *ondatra.DUTDevice, tunnelType string, location string) {
	input := util.SshRunCommand(t, dut, fmt.Sprintf("show ofa performance %s location %s", tunnelType, location))
	t.Log("command output using ssh\n", input)
	// todo: use gnmi to get the output instead of ssh
	// input := util.CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("show ofa performace %s location %s", tunnelType, location))
	// t.Log("command output using gnmi\n", input)
	client_view, server_view, err := parseOfaPerf(input)
	if err != nil {
		// suppressing Errorf to avoid test failure, sometimes the performance counters are not available
		// for the tunnel type after we clear the counters
		t.Logf("Error parsing ofa performance: %v", err)
	}
	t.Logf("Ofa performance for %s, client view: %v, server view: %v", tunnelType, client_view, server_view)
}

func clearOfaPerformance(t *testing.T, dut *ondatra.DUTDevice, tunnelType string, location string) {
	util.SshRunCommand(t, dut, fmt.Sprintf("clear ofa performance %s location %s", tunnelType, location))
}

func testDcGateScale(t *testing.T) {
	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)
	// waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")

	batches := 8
	// get free resource IDs
	var gridRsrc ResourceData

	t.Run("Get grid pool usage", func(t *testing.T) {
		gridRsrc = getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)
	})

	availableForUseResourceIDs := reduceToPercent(gridRsrc.AvailableResourceIDs, UsableResoucePercent)
	// plan resouce distribution among various tunnel types for DCGate profile
	t.Logf("Available for use resource IDs: %d", availableForUseResourceIDs)
	nhsPerNHG := 8 // for encap and decapEncap tunnels

	reserveForDecapF := 1000
	reserveForDecapV := 48
	reserveForDecapEncap := 512 // use closest value to it which can be equally divided in batches*nhsPerNHG
	nhgForDecapF := reserveForDecapF
	nhgForDecapV := reserveForDecapV
	avaialableForEncap := availableForUseResourceIDs - reserveForDecapF - reserveForDecapV - reserveForDecapEncap
	// get closest floor value of nhgs for encapDecap tunnels that can be divided in batches with nhsPerNHG size
	// pass on leftover values to be used by encap tunnels
	frr1NHG, frr1NHLeftover, _ := DivideAndAdjust(reserveForDecapEncap, nhsPerNHG, batches)

	// use frr1NHLeftovers for encap tunnels
	// distribute the available resource IDs to NHGs such that it can be divided in batches.
	// remaining resource IDs will be used to configure using a single NHG with nhs count = nhLeftover
	nhg, nhLeftover, _ := DivideAndAdjust(avaialableForEncap+frr1NHLeftover, nhsPerNHG, batches)
	t.Logf("Reserving resources %d for decapF, %d for decapV, %d for decapEncap, %d for encap", reserveForDecapF, reserveForDecapV, reserveForDecapEncap, avaialableForEncap+frr1NHLeftover)
	t.Logf("Possible NHG: %d, with NHsPerNHG: %d, leftover: %d with available %d resource IDs", nhg, nhsPerNHG, nhLeftover, avaialableForEncap)

	// build the DCGate profile
	gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths, numUniqueNHGs: batches, numNHPerNHG: 1}, //primary path
		&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG), numUniqueNHGs: batches, numNHPerNHG: 1},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: nhg / 2, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_A
		&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: nhg / 2, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_B
		&routesParam{segment: "Frr1Level1", nextHops: tcArgs.frr1Paths, numUniqueNHGs: 1, numNHPerNHG: 1},
		&routesParam{segment: "Frr1Level2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, frr1NHG*nhsPerNHG), numUniqueNHGs: frr1NHG, numNHPerNHG: nhsPerNHG},
		&routesParam{segment: "DecapWan", numUniqueNHGs: nhgForDecapF, numNHPerNHG: 1, ipEntries: iputil.GenerateIPs(IPBlockDecap, reserveForDecapF), nextHopWeight: generateNextHopWeights(64, 1)},
		&routesParam{segment: "DecapWanVar", numUniqueNHGs: nhgForDecapV, numNHPerNHG: 1},
	)

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	t.Log("Measure performance for each PrimaryUniqueIntfCard")
	for _, card := range pathInfo.PrimaryUniqueIntfCards {
		t.Logf("Processing PrimaryUniqueIntfCard: %v", card)
		gp.measurePerf = &tunTypes{location: card, tunType: []string{"iptnlnh", "iptnlencap", "iptnldecap"}}
	}

	t.Run("Push batch config", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
		time.Sleep(10 * time.Second)
	})

	t.Run("Configure leftover NHGs", func(t *testing.T) {
		// configure leftover NHGs
		if nhLeftover > 0 {
			if nhLeftover > MaxNhsPerNHG {
				// configure leftover NHGs
				t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3D", MaxNhsPerNHG)
				remaingNhGp1 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
					&routesParam{segment: "PrimaryLevel3D", numUniqueNHGs: 1, numNHPerNHG: MaxNhsPerNHG, nextHopWeight: generateNextHopWeights(256, MaxNhsPerNHG)},
				)
				remaingNhGp1.pushBatchConfig(t, tcArgs, []int{0})
				//leftover NHGs after deducting MaxNhsPerNHG
				nhLeftover -= MaxNhsPerNHG
			}
			t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3C", nhLeftover)
			remaingNhGp2 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
				&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 1, numNHPerNHG: nhLeftover, nextHopWeight: generateNextHopWeights(64, nhLeftover)},
			)
			remaingNhGp2.pushBatchConfig(t, tcArgs, []int{0})
		} else {
			t.Skip("no leftover NHGs to configure")
		}
	})

	t.Run("Resource consuption for all unique cards", func(t *testing.T) {
		getResouceConsumption(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, pathInfo.PrimaryUniqueIntfCards)
	})

	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1, 2, 3, 4, 5, 6, 7})
	})

	for _, batch := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("Validating decap traffic batch %d", batch), func(t *testing.T) {
			testDecapTrafficFlows(t, tcArgs, gp, []int{batch})
		})
	}

}

func testFlushAll(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	client.Start(ctx, t)

	t.Log("Flushing all gRIBI entries")
	if err := gribi.FlushAll(client); err != nil {
		t.Error(err)
	}
	time.Sleep(30 * time.Second)
	client.Stop(t)
}

func reduceToPercent(value int, percent int) int {
	return (value * percent) / 100
}

func getResouceConsumption(t *testing.T, dut *ondatra.DUTDevice, pool, bank int, rploc string, lcLoc []string) {
	cmdTemplates := []string{
		"show grid pool %d bank %d location %s",
		"show controller npu resources all location %s",
		"show controller npu debugshell 0 \"script resource_usage\" location %s",
		"show ofa objects iptnlencap object-count location %s",
		"show ofa objects iptnldecap object-count location %s",
		"show ofa objects iptnlnh object-count location %s",
	}

	// Run the rploc command only once
	cmd := fmt.Sprintf(cmdTemplates[0], pool, bank, rploc)
	util.SshRunCommand(t, dut, cmd)

	// Run the lcLoc commands for each location
	for _, loc := range lcLoc {
		for i := 1; i < len(cmdTemplates); i++ {
			cmd := fmt.Sprintf(cmdTemplates[i], loc)
			util.SshRunCommand(t, dut, cmd)
		}
	}
}

func waitForResoucesToRestore(t *testing.T, dut *ondatra.DUTDevice, pool, bank int, rploc, lcLoc string) {
	// wait upto 10 minutes for the resources to restore
	for i := 0; i < 11; i++ {
		if fibUsage := getGridPoolUsageViaGNMI(t, dut, pool, bank, rploc).ClientUsages["fib-mgr"]; fibUsage != 0 {
			t.Logf("Waiting for resources to be restored after %d minutes", i)
			time.Sleep(1 * time.Minute)
		} else {
			break
		}
	}
}

func testDcGateTriggers(t *testing.T) {

	// initial setting
	if gArgs == nil && !bConfig.isConfigured() {
		gArgs = configureBaseInfra(t, bConfig)
	}
	tcArgs := gArgs
	tcArgs.client.Start(tcArgs.ctx, t)

	// cleanup all existing gRIBI entries at the end of the test
	defer func(ta *testArgs) {
		if *flush_after {
			t.Log("Flushing all gRIBI entries at the end of the test")
			gribi.FlushAll(ta.client)
		}
	}(tcArgs)

	// cleanup all existing gRIBI entries in the begining of the test
	t.Log("Flush all gRIBI entries at the beginning of the test")
	if *flush_before {
		t.Log("Flushing all gRIBI entries at the beginning of the test")
		if err := gribi.FlushAll(tcArgs.client); err != nil {
			t.Error(err)
		}
		// Wait for the gribi entries get flushed
		waitForResoucesToRestore(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, "")
	}
	defer tcArgs.client.Stop(t)

	batches := 8
	vrfCount := 4
	// get free resource IDs
	gridRsrc := getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)
	getGridPoolUsageViaGNMI(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP)
	availableForUseResourceIDs := reduceToPercent(gridRsrc.AvailableResourceIDs, UsableResoucePercent)
	// availableForUseResourceIDs := 12000 - 145 //13136
	// plan resouce distribution among various tunnel types for DCGate profile
	t.Logf("Available for use resource IDs: %d", availableForUseResourceIDs)
	nhsPerNHG := 8 // for encap and decapEncap tunnels

	reserveForDecapF := 1024
	reserveForDecapV := 48
	reserveForDecapEncap := 704 // use closest value to it which can be equally divided in batches*nhsPerNHG
	nhgForDecapF := reserveForDecapF
	nhgForDecapV := reserveForDecapV
	avaialableForEncap := availableForUseResourceIDs - reserveForDecapF - reserveForDecapV - reserveForDecapEncap
	// get closest floor value of nhgs for encapDecap tunnels that can be divided in batches with nhsPerNHG size
	// pass on leftover values to be used by encap tunnels
	frr1NHG, frr1NHLeftover, _ := DivideAndAdjust(reserveForDecapEncap, nhsPerNHG, batches)
	t.Logf("Frr1NHGs: %d, frr1NHLeftover: %d from reserveForDecapEncap: %d", frr1NHG, frr1NHLeftover, reserveForDecapEncap)
	// use frr1NHLeftovers for encap tunnels
	// distribute the available resource IDs to NHGs such that it can be divided in batches.
	// remaining resource IDs will be used to configure using a single NHG with nhs count = nhLeftover
	nhg, nhLeftover, _ := DivideAndAdjust(avaialableForEncap+frr1NHLeftover, nhsPerNHG, batches*vrfCount)
	t.Logf("Reserving resources %d for decapF, %d for decapV, %d for decapEncap, %d for encap", reserveForDecapF, reserveForDecapV, reserveForDecapEncap, avaialableForEncap+frr1NHLeftover)
	t.Logf("Possible NHG: %d, with NHsPerNHG: %d, leftover: %d with available %d resource IDs", nhg, nhsPerNHG, nhLeftover, avaialableForEncap+frr1NHLeftover)

	// divide tcArgs.primaryPaths in halves to be used by primary and frr1 paths
	// mid := len(tcArgs.primaryPaths) / 2
	// build the DCGate profile
	// failing test case
	gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
		&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths, numUniqueNHGs: 512, numNHPerNHG: 8}, //primary path
		&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG), numUniqueNHGs: 256, numNHPerNHG: 2},
		&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: nhg / vrfCount, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_A
		&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: nhg / vrfCount, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_B
		&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: nhg / vrfCount, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_C
		&routesParam{segment: "PrimaryLevel3D", numUniqueNHGs: nhg / vrfCount, numNHPerNHG: nhsPerNHG}, // allocate half of the available nhgs for ENCAP_TE_VRF_D
		&routesParam{segment: "Frr1Level1", ipEntries: iputil.GenerateIPs(VipFrr1IPBlock, 64), nextHops: tcArgs.frr1Paths, numUniqueNHGs: 8, numNHPerNHG: 8},
		&routesParam{segment: "Frr1Level2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, nhg*nhsPerNHG), numUniqueNHGs: frr1NHG, numNHPerNHG: nhsPerNHG},
		&routesParam{segment: "DecapWan", numUniqueNHGs: nhgForDecapF, numNHPerNHG: 1, ipEntries: iputil.GenerateIPs(IPBlockDecap, reserveForDecapF), nextHopWeight: generateNextHopWeights(1, 1)},
		&routesParam{segment: "DecapWanVar", numUniqueNHGs: nhgForDecapV, numNHPerNHG: 1},
	)

	// working test case
	// gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
	// 	&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths[:mid], numUniqueNHGs: 512, numNHPerNHG: 8}, //primary path
	// 	&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, 12000), numUniqueNHGs: 256, numNHPerNHG: 2},
	// 	&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_A
	// 	&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_B
	// 	&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_C
	// 	&routesParam{segment: "PrimaryLevel3D", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_D
	// 	&routesParam{segment: "Frr1Level1", ipEntries: iputil.GenerateIPs(VipFrr1IPBlock, 64), nextHops: tcArgs.primaryPaths[mid:], numUniqueNHGs: 8, numNHPerNHG: 8},
	// 	&routesParam{segment: "Frr1Level2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, 12000), numUniqueNHGs: 8, numNHPerNHG: 8},
	// 	&routesParam{segment: "DecapWan", numUniqueNHGs: nhgForDecapF, numNHPerNHG: 1, ipEntries: iputil.GenerateIPs(IPBlockDecap, reserveForDecapF), nextHopWeight: generateNextHopWeights(1, 1)},
	// 	&routesParam{segment: "DecapWanVar", numUniqueNHGs: nhgForDecapV, numNHPerNHG: 1},
	// )

	// working case - best case to be projected to customer
	// gp := NewGribiProfile(t, batches, true, true, tcArgs.dut,
	// 	&routesParam{segment: "PrimaryLevel1", nextHops: tcArgs.primaryPaths[:mid], numUniqueNHGs: 512, numNHPerNHG: 8}, //primary path
	// 	&routesParam{segment: "PrimaryLevel2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, 6400), numUniqueNHGs: 256, numNHPerNHG: 2},
	// 	&routesParam{segment: "PrimaryLevel3A", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_A
	// 	&routesParam{segment: "PrimaryLevel3B", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_B
	// 	&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_C
	// 	&routesParam{segment: "PrimaryLevel3D", numUniqueNHGs: 200, numNHPerNHG: 8}, // define tunnel type for this vrf ENCAP_TE_VRF_D
	// 	&routesParam{segment: "Frr1Level1", ipEntries: iputil.GenerateIPs(VipFrr1IPBlock, 64), nextHops: tcArgs.primaryPaths[mid:], numUniqueNHGs: 8, numNHPerNHG: 8},
	// 	&routesParam{segment: "Frr1Level2", ipEntries: iputil.GenerateIPs(V4TunnelIPBlock, 6400), numUniqueNHGs: 88, numNHPerNHG: 8},
	// 	&routesParam{segment: "DecapWan", numUniqueNHGs: nhgForDecapF, numNHPerNHG: 1, ipEntries: iputil.GenerateIPs(IPBlockDecap, reserveForDecapF), nextHopWeight: generateNextHopWeights(1, 1)},
	// 	&routesParam{segment: "DecapWanVar", numUniqueNHGs: nhgForDecapV, numNHPerNHG: 1},
	// )

	t.Log("Start gRIBI client and become leader")
	tcArgs.client.StartSending(tcArgs.ctx, t)
	if err := awaitTimeout(tcArgs.ctx, tcArgs.client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for client: %v", err)
	}
	electionID := gribi.BecomeLeader(t, tcArgs.client)
	t.Logf("Election ID: %v", electionID)

	t.Log("Measure performance for each PrimaryUniqueIntfCard")
	for _, card := range pathInfo.PrimaryUniqueIntfCards {
		t.Logf("Processing PrimaryUniqueIntfCard: %v", card)
		gp.measurePerf = &tunTypes{location: card, tunType: []string{"iptnlnh", "iptnlencap", "iptnldecap"}}
	}

	t.Run("Push gribi batch config", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
		t.Logf("Waiting for 10 seconds for the gRIBI entries to be programmed")
		time.Sleep(10 * time.Second)
	})

	// configure leftover NHGs
	if nhLeftover > 0 {

		// if nhLeftover > nhsPerNHG {
		// 	// configure leftover NHGs
		// 	completeNHGs := nhLeftover / nhsPerNHG
		// 	nhLeftover = nhLeftover % nhsPerNHG
		// 	t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3D", MaxNhsPerNHG)
		// 	remaingNhGp1 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
		// 		&routesParam{segment: "PrimaryLevel3D", numUniqueNHGs: completeNHGs, numNHPerNHG: nhsPerNHG, nextHopWeight: generateNextHopWeights(L3Weight, nhsPerNHG)},
		// 	)
		// 	remaingNhGp1.pushBatchConfig(t, tcArgs, []int{0})
		// }

		// if nhLeftover > 0 {
		// 	t.Logf("Configuring remaining NHs: %d, using vrf PrimaryLevel3C", nhLeftover)
		// 	remaingNhGp2 := NewGribiProfile(t, 1, false, false, tcArgs.dut,
		// 		&routesParam{segment: "PrimaryLevel3C", numUniqueNHGs: 1, numNHPerNHG: nhLeftover, nextHopWeight: generateNextHopWeights(L3Weight, nhLeftover)},
		// 	)
		// 	remaingNhGp2.pushBatchConfig(t, tcArgs, []int{0})
		// }
	}

	t.Run("Resource consuption for all unique cards", func(t *testing.T) {
		getResouceConsumption(t, tcArgs.dut, 1, 4, tcArgs.DUT.ActiveRP, pathInfo.PrimaryUniqueIntfCards)
	})

	t.Run("Validating encap traffic", func(t *testing.T) {
		testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1, 2, 3, 4, 5, 6, 7}, &ConvOptions{measureConvergence: true})

	})

	t.Run("Validating /32 decap traffic", func(t *testing.T) {
		for _, b := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
			batch := b // capture range variable
			t.Run(fmt.Sprintf("Batch%d", batch), func(t *testing.T) {
				for _, encap := range []string{"A", "B", "C", "D"} {
					encapType := encap // capture range variable
					t.Run(fmt.Sprintf("Encap%s", encapType), func(t *testing.T) {
						testDecapTrafficFlowsForEncap(t, tcArgs, gp, []int{batch}, []string{encapType})
					})
				}
			})
		}
	})

	for i, batches := range [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7}} {
		batchLabel := fmt.Sprintf("%v", batches)
		t.Run(fmt.Sprintf("Validating variable prefix decap traffic for batches %s", batchLabel), func(t *testing.T) {
			// Running convergence test for first bath (0,1,2) only to save time
			if i == 0 {
				testDecapTrafficFlowsForVariablePrefix(t, tcArgs, gp, batches, []string{"A", "B", "C", "D"}, &ConvOptions{measureConvergence: true})
			} else {
				testDecapTrafficFlowsForVariablePrefix(t, tcArgs, gp, batches, []string{"A", "B", "C", "D"})
			}
		})
	}
	// for _, b := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
	// 	for _, encap := range []string{"A", "B", "C", "D"} {
	// 		testDecapTrafficFlowsForVariablePrefix(t, tcArgs, gp, []int{b}, []string{encap})
	// 	}
	// }

	// testDecapTrafficFlows(t, tcArgs, gp, []int{1})
	// testDecapTrafficFlows(t, tcArgs, gp, []int{2})

	t.Run("Delete all gribi batch configurations", func(t *testing.T) {
		gp.DeleteBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
		time.Sleep(10 * time.Second)
	})
	t.Run("Re-add all gribi batch configurations", func(t *testing.T) {
		gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
		time.Sleep(10 * time.Second)
	})

	if tcArgs.DUT.DualSup {
		// remove hardware
		configureHwModuleIPTunnelConfig(t, tcArgs.DUT.Device, true)
		// verify no oor
		verifyResourcesNotInRed(t, tcArgs.DUT.Device, pathInfo.PrimaryUniqueIntfCards)
		// reload hardware
		for _, lc := range pathInfo.PrimaryUniqueIntfCards {
			hautils.DoLcReboot(t, tcArgs.DUT.Device, lc)
		}
		// verify oor
		verifyResourcesInRed(t, tcArgs.DUT.Device, pathInfo.PrimaryUniqueIntfCards)
		// re-add hardware
		configureHwModuleIPTunnelConfig(t, tcArgs.DUT.Device, false)
		// verify oor
		verifyResourcesInRed(t, tcArgs.DUT.Device, pathInfo.PrimaryUniqueIntfCards)
		// reload hardware
		for _, lc := range pathInfo.PrimaryUniqueIntfCards {
			hautils.DoLcReboot(t, tcArgs.DUT.Device, lc)
		}
		// verify oor false
		verifyResourcesNotInRed(t, tcArgs.DUT.Device, pathInfo.PrimaryUniqueIntfCards)
	}

	// Iterate over each trigger and run it as a subtest
	triggers = append([]Trigger{
		{
			name: "DELETE-RE-ADD",
			fn: func(ctx context.Context, t *testing.T) {
				gp.DeleteBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
				time.Sleep(10 * time.Second)
				gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
			},
			duration:              10 * time.Minute,
			reprogrammingRequired: false,
			reconnectClient:       false,
		},
	}, triggers...)
	for _, trigger := range triggers {
		t.Run(trigger.name, func(t *testing.T) {

			trigger.fn(tcArgs.ctx, t)
			time.Sleep(trigger.duration) // Use the duration specified in the trigger

			// Collect logs after each trigger
			t.Logf("LogCollectionAfterTrigger%s", trigger.name)
			log_collector.CollectRouterLogs(tcArgs.ctx, t, tcArgs.DUT.Device, tcArgs.LogDir, "LogCollectionAfterTrigger"+trigger.name, tcArgs.CommandPatterns)

			if trigger.reconnectClient {
				t.Log("Reconnect clients")
				// tcArgs.ReconnectClients(t, 30)
			}

			// If reprogramming is required, run the Reprogramming function
			if trigger.reprogrammingRequired {
				t.Run("Reprogramming", func(t *testing.T) {
					gp.pushBatchConfig(t, tcArgs, []int{0, 1, 2, 3, 4, 5, 6, 7})
					time.Sleep(10 * time.Second)
				})
			}
			// verify traffic after the trigger
			t.Run(fmt.Sprintf("Validating encap traffic after trigger %s", trigger.name), func(t *testing.T) {
				testEncapTrafficFlows(t, tcArgs, gp, []int{0, 1, 2, 3, 4, 5, 6, 7})
			})

			t.Run(fmt.Sprintf("Validating decap traffic after trigger (0,1,2,3,4,5,6,7)x(A,B,C,D) %s", trigger.name), func(t *testing.T) {
				for _, b := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
					batch := b // capture range variable
					t.Logf("Validating decap Batch%d", batch)
					for _, encap := range []string{"A", "B", "C", "D"} {
						encapType := encap // capture range variable
						t.Logf("Validating decap Encap%s", encapType)
						testDecapTrafficFlowsForEncap(t, tcArgs, gp, []int{batch}, []string{encapType})
					}
				}
			})

			for _, batches := range [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7}} {
				batchLabel := fmt.Sprintf("%v", batches)
				t.Run(fmt.Sprintf("Validating variable prefix decap traffic after trigger %s for batches %s", trigger.name, batchLabel), func(t *testing.T) {
					testDecapTrafficFlowsForVariablePrefix(t, tcArgs, gp, batches, []string{"A", "B", "C", "D"})
				})
			}

			// Collect logs after gribi programing
			t.Logf("LogCollectionAfterTrafficValidation %s", trigger.name)
			log_collector.CollectRouterLogs(tcArgs.ctx, t, tcArgs.DUT.Device, tcArgs.LogDir, "LogCollectionAfterTrafficValidation"+trigger.name, tcArgs.CommandPatterns)

		})
	}
}

func configureHwModuleIPTunnelConfig(t *testing.T, dut *ondatra.DUTDevice, remove bool) {
	t.Log("Removing hardware")
	// util.SshRunCommand(t, dut, "hw-module profile cef iptunnel scale")
	batchSet := &gnmi.SetBatch{}
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	cliCfg := "hw-module profile cef iptunnel scale\n"
	if remove {
		cliCfg = "no hw-module profile cef iptunnel scale\n"
	}
	gnmi.BatchUpdate(batchSet, cliPath, cliCfg)
}

type ResourceEntry struct {
	Resource    string
	MaxEntries  string
	UsedEntries string
}

func isResourcesInRed(t *testing.T, dut *ondatra.DUTDevice, lcLoc string) bool {
	entries := GetResourcesInOORState(t, dut, lcLoc)
	inRed := false
	if len(entries) > 0 {
		inRed = true
		t.Logf("Resources in Red state on %s:", lcLoc)
		for _, entry := range entries {
			t.Logf("Resource: %s, MaxEntries: %s, UsedEntries: %s", entry.Resource, entry.MaxEntries, entry.UsedEntries)
		}
	} else {
		t.Logf("No resources in Red state on %s", lcLoc)

	}
	return inRed
}

// Wrapper function to verify resources are in red after reboot
func verifyResourcesInRed(t *testing.T, dut *ondatra.DUTDevice, lcs []string) {
	for _, lc := range lcs {
		if !isResourcesInRed(t, dut, lc) {
			t.Fatalf("resources are not in red after rebooting LC %v", lc)
		}
	}
}

// Wrapper function to verify resources are NOT in red before reboot
func verifyResourcesNotInRed(t *testing.T, dut *ondatra.DUTDevice, lcs []string) {
	for _, lc := range lcs {
		if isResourcesInRed(t, dut, lc) {
			t.Fatalf("resources are in red before rebooting LC %v", lc)
		}
	}
}

func GetResourcesInOORState(t *testing.T, dut *ondatra.DUTDevice, lcLoc string) []ResourceEntry {
	cmd := fmt.Sprintf("show controller npu debugshell 0 \"script resource_usage\" location %s", lcLoc)
	cliOutput := util.SshRunCommand(t, dut, cmd)
	scanner := bufio.NewScanner(strings.NewReader(cliOutput))
	var results []ResourceEntry
	var inTable bool

	// Start scanning line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Detect when the table starts
		if strings.HasPrefix(line, "+-------------------------------------------------+") {
			if !inTable {
				inTable = true
			} else {
				// End of table
				break
			}
			continue
		}

		// Skip lines that are not part of the table
		if !inTable || strings.HasPrefix(line, "|                     Resource                    ") {
			continue
		}

		// Parse the table rows
		fields := strings.Split(line, "|")
		if len(fields) > 6 {
			resource := strings.TrimSpace(fields[1])
			maxEntries := strings.TrimSpace(fields[4])
			usedEntries := strings.TrimSpace(fields[5])
			state := strings.TrimSpace(fields[6])

			// Check if state is Red
			if state == "Red" {
				results = append(results, ResourceEntry{
					Resource:    resource,
					MaxEntries:  maxEntries,
					UsedEntries: usedEntries,
				})
			}
		}
	}

	return results
}
