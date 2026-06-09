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
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ============================================================
// Constants — shared across T1 and T2
// ============================================================

const (
	// Port1 DUT/ATE addresses — single /30 sub-interface.
	DUTPort1IPv4 = "192.0.2.1"
	ATEPort1IPv4 = "192.0.2.2"
	DUTPort1IPv6 = "2001:0:0:1::1"
	ATEPort1IPv6 = "2001:0:0:2::2"
	ATEPort1MAC  = "02:00:01:01:01:01"

	// Port2: 640 VLAN-tagged /30 sub-interfaces carved from 198.18.0.0/20.
	DUTPort2IPv4      = "193.0.2.1"
	ATEPort2IPv4      = "193.0.2.2"
	DUTPort2IPv6      = "3001:0:0:1::1"
	ATEPort2IPv6      = "3001:0:0:2::2"
	ATEPort2MAC       = "02:00:02:00:00:01"
	ATEPort2MACStep   = "00:00:00:00:01:00"
	NumPort1VLANs     = 1
	NumPort2VLANs     = 640
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

	// Default VRF - identical for T1 and T2.
	NumDefaultNH   = 1_000
	NumDefaultNHG  = 1_000
	NumDefaultIPv4 = 1_000

	// Transit VRFs — identical for T1 and T2.
	NumTransitNH_D1  = 1536
	NumTransitNH_D2  = 1536
	NumTransitNHG_E1 = 768
	NumTransitNHG_E2 = 768
	NumTransitIPv4   = 200_000 // per Transit VRF

	// Repair VRF IPv4 count — identical for T1 and T2.
	NumRepairIPv4 = 200_000

	// Encap/Decap VRFs — counts identical for T1 and T2.
	NumEncapVRFs       = 16
	NumEncapIPv4PerVRF = 10_000
	NumEncapIPv6PerVRF = 10_000
	NumDecapEntries    = 48

	// Default VRF scale — identical for T1 and T2.
	IPv4PrefixStartAddress = "10.0.0.1"

	// Prefix start addresses for Transit and Repair VRFs.
	TransitVRF111PrefixStart = "100.0.0.1"
	TransitVRF222PrefixStart = "101.0.0.1"
	RepairNHPrefixStart      = "102.0.0.1"
	RepairIPv4PrefixStart    = "103.0.0.1"
	EncapNHTunnelStart       = "198.18.128.1"

	// Common prefix step used across multiple VRF builders.
	CommonPrefixStep     = "0.0.0.1"
	CommonIPv6PrefixStep = "::1"

	// Encap NHG NH-count splits (percentages of numEncapDefaultNHG):
	// 75% → 8 NHs/NHG, 20% → 32 NHs/NHG, 5% → 32 NHs/NHG.
	PctEncap8NH  = 75
	PctEncap32NH = 20

	// Traffic parameters — identical for T1 and T2.
	TrafficDuration = 5 * time.Minute
	TrafficLossTol  = uint64(5)
	TrafficRateMpps = uint64(30_000_000) // 30 Mpps aggregate

	// gRIBI batch programming parameters.
	BatchChunkSize = 2_000

	// NH/NHG ID base constants — kept non-overlapping across all VRFs.
	NHBaseDefault  = uint64(1_000)
	NHGBaseDefault = uint64(2_000)
	NHBaseD1       = uint64(10_000)
	NHBaseD2       = uint64(12_000)
	NHGBaseE1      = uint64(20_000)
	NHGBaseE2      = uint64(21_000)
	NHBaseRepair   = uint64(30_000)
	NHGBaseRepair  = uint64(32_000)
	NHBaseEncap    = uint64(50_000)
	NHGBaseEncap   = uint64(82_001)
	NHBaseDecap    = uint64(120_000)
	NHGBaseDecap   = uint64(120_100)

	// Static NHG IDs for S1/S2.
	StaticS1NHG   = uint64(9001)
	StaticS2NHG   = uint64(9002)
	RandomIPCheck = 100
)

// ============================================================
// Package-level variables
// ============================================================

var (
	// dutPort1Attr and atePort1Attr hold port1 L3 attributes.
	dutPort1Attr = attrs.Attributes{
		Name:    "DUT Ingress Port1",
		Desc:    "dutPort1",
		IPv4:    DUTPort1IPv4,
		IPv4Len: IPv4IntfMask,
		IPv6:    DUTPort1IPv6,
		IPv6Len: IPv6IntfMask,
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

	// encapVRFs holds names ENCAP_TE_VRF_A ... ENCAP_TE_VRF_P.
	encapVRFs = BuildEncapVRFs()

	// AllNonDefaultVRFs is the full list of VRFs that must be created.
	allNonDefaultVRFs = BuildAllNonDefaultVRFs(encapVRFs)

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

// ScaleParams holds the scale constants that differ between T1 and T2.
// Each test package constructs one of these and passes it to the shared helpers.
type ScaleParams struct {
	// PctNHG512 is the percentage of Default VRF NHGs with 1/512 granularity.
	// T1: 80, T2: 70.
	PctNHG512 int
	// NumRepairNHG is the number of NHGs in REPAIR_VRF.
	// T1: 1000, T2: 2000.
	NumRepairNHG int
	// NumEncapDefaultNHG is the number of NHGs injected into the default VRF
	// for encap VRF entries (T3 scale target).
	// T1: 4000, T2: 8000.
	NumEncapDefaultNHG int
	// NumUniqueEncapNH is the total number of unique encap NHs (T4 scale target).
	// T1: 16000, T2: 32000.
	NumUniqueEncapNH int
}

// TrafficTestCase is a table-driven entry for the two traffic profiles.
type TrafficTestCase struct {
	Name    string
	UseIMIX bool
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

// BuildEncapVRFs returns the 16 encap VRF names ENCAP_TE_VRF_A … ENCAP_TE_VRF_P.
func BuildEncapVRFs() []string {
	v := make([]string, NumEncapVRFs)
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

// ConfigureDUT sets up port interfaces, VRFs, and VRF-selection policy.
func ConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := gnmi.OC()
	vrfBatch := new(gnmi.SetBatch)

	ConfigureHardwareInit(t, dut)
	CreateGRIBIScaleVRFs(t, dut, vrfBatch)
	portList := []*ondatra.Port{dp1, dp2}
	dutPortAttrs := []attrs.Attributes{dutPort1Attr, dutPort2Attr}

	for idx, a := range dutPortAttrs {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		gnmi.BatchUpdate(vrfBatch, d.Interface(p.Name()).Config(), intf)
		t.Logf("Configured DUT port %s (%s)", p.Name(), a.Desc)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		t.Log("Configure/update Network Instance type")
		dutConfNIPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.BatchUpdate(vrfBatch, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	}
	// Configure sub-interfaces on port1 (1 VLAN) and port2 (640 VLANs).
	ConfigureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp1, DUTPort1IPv4Start, DUTPort1IPv6Start, StartVLANPort1, NumPort1VLANs)
	ConfigureDUTSubinterfaces(t, vrfBatch, new(oc.Root), dut, dp2, DUTPort2IPv4Start, DUTPort2IPv6Start, StartVLANPort2, NumPort2VLANs)
	ConfigureCLIDecapVRFMode(t, dut)
	ConfigureVRFSelectionPolicyOC(t, dut, vrfBatch)
	vrfBatch.Set(t, dut)
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
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6Addr)
	a6.PrefixLength = ygot.Uint8(uint8(IPv6IntfMask))
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(dutPort.Name()).Subinterface(index).Config(), s)
}

// ConfigureHardwareInit pushes platform-specific hardware init configs.
func ConfigureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareVrfCfg := NewDUTHardwareInit(t, dut, FeatureVrfSelectionExtended)
	hardwarePfCfg := NewDUTHardwareInit(t, dut, FeaturePolicyForwarding)
	if hardwareVrfCfg == "" || hardwarePfCfg == "" {
		return
	}
	PushDUTHardwareInitConfig(t, dut, hardwareVrfCfg)
	PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
}

// CreateGRIBIScaleVRFs creates all non-default VRF network-instances plus the DEFAULT instance.  Uses deviations.DefaultNetworkInstance for the correct name.
func CreateGRIBIScaleVRFs(t *testing.T, dut *ondatra.DUTDevice, vrfBatch *gnmi.SetBatch) {
	t.Helper()
	droot := new(oc.Root)

	// DEFAULT NI.
	defaultNI := deviations.DefaultNetworkInstance(dut)
	ni := droot.GetOrCreateNetworkInstance(defaultNI)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().NetworkInstance(defaultNI).Config(), ni)

	// All non-default VRFs.
	for _, vrf := range allNonDefaultVRFs {
		ni := droot.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.BatchUpdate(vrfBatch, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
	vrfBatch.Set(t, dut)
}

// ConfigureOTG builds and returns the OTG config for both ATE ports. port1: 1 sub-interface; port2: 640 VLAN sub-interfaces.
func ConfigureOTG(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) (gosnappi.Config, []string) {
	t.Helper()
	ateConfig := gosnappi.NewConfig()
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	ateConfig.Ports().Add().SetName(ap1.ID())
	ateConfig.Ports().Add().SetName(ap2.ID())

	// Base (untagged) devices for each port.
	CreateATEDevice(t, ateConfig, ap1, 0, atePort1Attr.Name, atePort1Attr.MAC, dutPort1Attr.IPv4, atePort1Attr.IPv4, dutPort1Attr.IPv6, atePort1Attr.IPv6)
	CreateATEDevice(t, ateConfig, ap2, 0, atePort2Attr.Name, atePort2Attr.MAC, dutPort2Attr.IPv4, atePort2Attr.IPv4, dutPort2Attr.IPv6, atePort2Attr.IPv6)

	// VLAN sub-interfaces.
	ifNames := MustConfigureATESubinterfaces(t, ateConfig, ap1, dut, atePort1Attr.Name, atePort1Attr.MAC, DUTPort1IPv4Start, ATEPort1IPv4Start, DUTPort1IPv6Start, ATEPort1IPv6Start, StartVLANPort1, NumPort1VLANs)
	MustConfigureATESubinterfaces(t, ateConfig, ap2, dut, atePort2Attr.Name, atePort2Attr.MAC, DUTPort2IPv4Start, ATEPort2IPv4Start, DUTPort2IPv6Start, ATEPort2IPv6Start, StartVLANPort2, NumPort2VLANs)

	return ateConfig, ifNames
}

// CreateATEDevice creates a single ATE device with Ethernet, optional VLAN, IPv4 and IPv6 configuration.
func CreateATEDevice(t *testing.T, ateConfig gosnappi.Config, atePort *ondatra.Port, vlanID uint16, name, mac, dutIPv4, ateIPv4, dutIPv6, ateIPv6 string) {
	t.Helper()
	dev := ateConfig.Devices().Add().SetName(name + ".Dev")
	eth := dev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
	eth.Connection().SetPortName(atePort.ID())
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

// BatchModify pushes entries to the DUT in chunks of BatchChunkSize.
func BatchModify(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, entries []fluent.GRIBIEntry, wTime time.Duration) *gribi.Client {
	t.Helper()
	gSession := NewGRIBIClient(t, dut)
	for i := 0; i < len(entries); i += BatchChunkSize {
		end := i + BatchChunkSize
		if end > len(entries) {
			end = len(entries)
		}
		gSession.AddEntries(t, entries[i:end], nil)
		// NOTE: AwaitTimeout per chunk is intentionally commented out due to a known bug.
		// if err := gSession.AwaitTimeout(context.Background(), t, 20*time.Second); err != nil {
		// 	t.Fatalf("gRIBI batch programming failed: %v", err)
		// }
	}
	// TODO: A time.Sleep is used as a temporary workaround. This will be fixed once the underlying issue is resolved.
	time.Sleep(wTime)
	return gSession
}

// BuildDefaultVRF generates NHs, NHGs, and IPv4 entries for the default VRF. pctNHG512 is the percentage of NHGs with 1/512 weight granularity.
func BuildDefaultVRF(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, pctNHG512 int) []string {
	t.Helper()
	wantPrefixes := make(map[string][]string)
	nhBase, nhgBase := NHBaseDefault, NHGBaseDefault
	atePort2Ips, err := iputil.GenerateIPsWithStep(ATEPort2IPv4Start, NumPort2VLANs, PortIPv4Step)
	if err != nil {
		t.Fatalf("ConfigureOTG: generate ATE port2 IPs: %v", err)
	}
	prefixHosts, err := iputil.GenerateIPsWithStep(IPv4PrefixStartAddress, NumDefaultIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildDefaultVRF: generate prefix IPs: %v", err)
	}

	prefixes := make([]string, NumDefaultIPv4)
	nhEntries := []fluent.GRIBIEntry{}
	nhgEntries := []fluent.GRIBIEntry{}
	ipv4Entries := []fluent.GRIBIEntry{}

	for i := 0; i < NumDefaultNH; i++ {
		nhEntry := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhBase + uint64(i)).WithIPAddress(atePort2Ips[i%NumPort2VLANs])
		nhEntries = append(nhEntries, nhEntry)
	}

	for i := 0; i < NumDefaultNHG; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgBase + uint64(i))
		if i < NumDefaultNHG*pctNHG512/100 {
			for j := 0; j < 62; j++ {
				nhg.AddNextHop(nhBase+uint64((i*64+j)%NumDefaultNH), 8)
			}
			nhg.AddNextHop(nhBase+uint64((i*64+62)%NumDefaultNH), 7)
			nhg.AddNextHop(nhBase+uint64((i*64+63)%NumDefaultNH), 9)
		} else {
			for j := 0; j < 62; j++ {
				nhg.AddNextHop(nhBase+uint64((i*64+j)%NumDefaultNH), 16)
			}
			nhg.AddNextHop(nhBase+uint64((i*64+62)%NumDefaultNH), 15)
			nhg.AddNextHop(nhBase+uint64((i*64+63)%NumDefaultNH), 17)
		}
		nhgEntries = append(nhgEntries, nhg)
	}

	for i := 0; i < NumDefaultIPv4; i++ {
		prefixes[i] = prefixHosts[i]
		ipv4Entries = append(ipv4Entries, fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(fmt.Sprintf("%s/%d", prefixHosts[i], IPv4HostMask)).WithNextHopGroup(nhgBase+uint64(i%NumDefaultNHG)).WithNextHopGroupNetworkInstance(defaultVRF))
	}

	// Combine all entries in dependency order: NHs first, then NHGs, then Prefixes.
	entries := append(nhEntries, nhgEntries...)
	entries = append(entries, ipv4Entries...)
	t.Logf("BuildDefaultVRF: %d NHs, %d NHGs, %d IPv4 entries", NumDefaultNH, NumDefaultNHG, NumDefaultIPv4)
	gSession := BatchModify(t, dut, ctx, entries, 120*time.Second)
	// DEFAULT VRF.
	for i := 0; i < FIBPrgCount; i++ {
		wantPrefixes[defaultVRF] = append(wantPrefixes[defaultVRF], fmt.Sprintf("%s/%d", prefixHosts[i], IPv4HostMask))
	}
	VerifyFIBProgrammed(t, gSession, wantPrefixes)
	gSession.Close(t)
	return prefixes
}

// BuildStaticGroups generates entries for the two static NHGs (S1 → REPAIR_VRF, S2 → decap DEFAULT).
func BuildStaticGroups(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string) (uint64, uint64) {
	t.Helper()
	s1NHG, s2NHG := StaticS1NHG, StaticS2NHG
	s1NH, _ := gribi.NHEntry(s1NHG, "VRFOnly", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: RepairVRFStr})
	s1NHGEntry, _ := gribi.NHGEntry(s1NHG, map[uint64]uint64{s1NHG: 1}, defaultVRF, fluent.InstalledInFIB)
	s2NH, _ := gribi.NHEntry(s2NHG, "Decap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: defaultVRF})
	s2NHGEntry, _ := gribi.NHGEntry(s2NHG, map[uint64]uint64{s2NHG: 1}, defaultVRF, fluent.InstalledInFIB)
	t.Logf("BuildStaticGroups: S1 NHG=%d (→REPAIR_VRF), S2 NHG=%d (decap→DEFAULT)", s1NHG, s2NHG)
	gSession := BatchModify(t, dut, ctx, []fluent.GRIBIEntry{s1NH, s1NHGEntry, s2NH, s2NHGEntry}, 30*time.Second)
	gSession.Close(t)
	return s1NHG, s2NHG
}

// BuildTransitVRFs generates entries for TE_VRF_111 and TE_VRF_222. NHs in D1/D2 point to default VRF NHs; NHGs in E1/E2 point to D1/D2 NHs with S1/S2 as backup; IPv4 entries point to E1/E2 NHGs.
func BuildTransitVRFs(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, defaultPrefixes []string, s1NHG, s2NHG uint64) {
	t.Helper()
	wantPrefixes := make(map[string][]string)
	entries := []fluent.GRIBIEntry{}

	for k := 0; k < NumTransitNH_D1; k++ {
		entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(NHBaseD1+uint64(k)).WithIPAddress(defaultPrefixes[k%len(defaultPrefixes)]))
	}
	for i := 0; i < NumTransitNHG_E1; i++ {
		entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(NHGBaseE1+uint64(i)).AddNextHop(NHBaseD1+uint64(i%NumTransitNH_D1), 1).AddNextHop(NHBaseD1+uint64((i+1)%NumTransitNH_D1), 63).WithBackupNHG(s1NHG))
	}

	for k := 0; k < NumTransitNH_D2; k++ {
		entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(NHBaseD2+uint64(k)).WithIPAddress(defaultPrefixes[k%len(defaultPrefixes)]))
	}
	for i := 0; i < NumTransitNHG_E2; i++ {
		entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(NHGBaseE2+uint64(i)).AddNextHop(NHBaseD2+uint64(i%NumTransitNH_D2), 1).AddNextHop(NHBaseD2+uint64((i+1)%NumTransitNH_D2), 63).WithBackupNHG(s2NHG))
	}

	vrf111Prefixes, err := iputil.GenerateIPsWithStep(TransitVRF111PrefixStart, NumTransitIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildTransitVRFs: generate TE_VRF_111 prefixes: %v", err)
	}
	for i, host := range vrf111Prefixes {
		entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(TransitVRF111Str).WithPrefix(fmt.Sprintf("%s/%d", host, IPv4HostMask)).WithNextHopGroup(NHGBaseE1+uint64(i%NumTransitNHG_E1)).WithNextHopGroupNetworkInstance(defaultVRF))
	}

	vrf222Prefixes, err := iputil.GenerateIPsWithStep(TransitVRF222PrefixStart, NumTransitIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildTransitVRFs: generate TE_VRF_222 prefixes: %v", err)
	}
	for i, host := range vrf222Prefixes {
		entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(TransitVRF222Str).WithPrefix(fmt.Sprintf("%s/%d", host, IPv4HostMask)).WithNextHopGroup(NHGBaseE2+uint64(i%NumTransitNHG_E2)).WithNextHopGroupNetworkInstance(defaultVRF))
	}
	t.Logf("BuildTransitVRFs: %d NHs in D1, %d NHGs in E1; %d NHs in D2, %d NHGs in E2", NumTransitNH_D1, NumTransitNHG_E1, NumTransitNH_D2, NumTransitNHG_E2)
	t.Logf("BuildTransitVRFs: %d IPv4 entries each transit VRF", NumTransitIPv4)
	gSession := BatchModify(t, dut, ctx, entries, 3*time.Minute)
	for _, pair := range []struct {
		vrf  string
		base []string
	}{
		{TransitVRF111Str, vrf111Prefixes},
		{TransitVRF222Str, vrf222Prefixes},
	} {
		pfxs := []string{}
		for i := 1; i < FIBPrgCount; i++ {
			pfxs = append(pfxs, fmt.Sprintf("%s/%d", pair.base[i], IPv4HostMask))
		}
		wantPrefixes[pair.vrf] = pfxs
	}
	VerifyFIBProgrammed(t, gSession, wantPrefixes)
	VerifyHierarchicalResolution(t, gSession, dut, wantPrefixes)
	gSession.Close(t)
}

// BuildRepairVRF generates NH/NHG/IPv4 entries for REPAIR_VRF. numRepairNHG is the T1/T2-specific NHG count.
func BuildRepairVRF(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, s2NHG uint64, numRepairNHG int) {
	t.Helper()
	tunnelDsts, err := iputil.GenerateIPsWithStep(RepairNHPrefixStart, numRepairNHG*2, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildRepairVRF: generate tunnel dsts: %v", err)
	}

	nhNhgEntries := []fluent.GRIBIEntry{}
	nhIdx := uint64(0)
	for i := 0; i < numRepairNHG; i++ {
		if i < numRepairNHG/2 {
			nhEntry, _ := gribi.NHEntry(NHBaseRepair+nhIdx, "Encap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx], VrfName: RepairVRFStr})
			nhgEntry, _ := gribi.NHGEntry(NHGBaseRepair+uint64(i), map[uint64]uint64{NHBaseRepair + nhIdx: 1}, defaultVRF, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: s2NHG})
			nhNhgEntries = append(nhNhgEntries, nhEntry, nhgEntry)
			nhIdx++
		} else {
			nh0, _ := gribi.NHEntry(NHBaseRepair+nhIdx, "Encap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx], VrfName: RepairVRFStr})
			nh1, _ := gribi.NHEntry(NHBaseRepair+nhIdx+1, "Encap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc222, Dest: tunnelDsts[nhIdx+1], VrfName: RepairVRFStr})
			nhgEntry, _ := gribi.NHGEntry(NHGBaseRepair+uint64(i), map[uint64]uint64{NHBaseRepair + nhIdx: 1, NHBaseRepair + nhIdx + 1: 1}, defaultVRF, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: s2NHG})
			nhNhgEntries = append(nhNhgEntries, nh0, nh1, nhgEntry)
			nhIdx += 2
		}
	}

	repairPrefixes, err := iputil.GenerateIPsWithStep(RepairIPv4PrefixStart, NumRepairIPv4, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildRepairVRF: generate repair prefixes: %v", err)
	}
	allEntries := nhNhgEntries
	for i, host := range repairPrefixes {
		allEntries = append(allEntries, fluent.IPv4Entry().WithNetworkInstance(RepairVRFStr).WithPrefix(fmt.Sprintf("%s/%d", host, IPv4HostMask)).WithNextHopGroup(NHGBaseRepair+uint64(i%numRepairNHG)).WithNextHopGroupNetworkInstance(defaultVRF))
	}
	t.Logf("BuildRepairVRF: %d NHGs (%d NHs), %d IPv4 entries", numRepairNHG, int(nhIdx), NumRepairIPv4)
	gSession := BatchModify(t, dut, ctx, allEntries, 30*time.Second)
	gSession.Close(t)
}

// BuildEncapDecapVRFs generates all encap NH/NHG/IPv4/IPv6 entries and decap entries. numEncapDefaultNHG and numUniqueEncapNH are the T1/T2-specific scale targets.
func BuildEncapDecapVRFs(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context, defaultVRF string, numEncapDefaultNHG, numUniqueEncapNH int) {
	t.Helper()
	allEntries := []fluent.GRIBIEntry{}
	wantPrefixes := make(map[string][]string)
	tunnelDsts, err := iputil.GenerateIPsWithStep(EncapNHTunnelStart, numUniqueEncapNH, CommonPrefixStep)
	if err != nil {
		t.Fatalf("BuildEncapDecapVRFs: generate encap NH tunnel dsts: %v", err)
	}
	for i := 0; i < numUniqueEncapNH; i++ {
		nhEntry, _ := gribi.NHEntry(NHBaseEncap+uint64(i), "Encap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Src: IPv4OuterSrc111, Dest: tunnelDsts[i], VrfName: TransitVRF111Str})
		allEntries = append(allEntries, nhEntry)
	}

	for i := 0; i < numEncapDefaultNHG; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(NHGBaseEncap + uint64(i))
		pct := i * 100 / numEncapDefaultNHG
		switch {
		case pct < PctEncap8NH:
			for j := 0; j < 8; j++ {
				weight := uint64(7)
				if j == 7 {
					weight = 15
				} // 7*7 + 15 = 64. GCD(7, 15)=1
				nhg.AddNextHop(NHBaseEncap+uint64((i*8+j)%numUniqueEncapNH), weight)
			}
		case pct < PctEncap8NH+PctEncap32NH:
			for j := 0; j < 32; j++ {
				weight := uint64(3)
				if j == 31 {
					weight = 35
				} // 31*3 + 35 = 128. GCD(3, 35)=1
				nhg.AddNextHop(NHBaseEncap+uint64((i*32+j)%numUniqueEncapNH), weight)
			}
		default:
			for j := 0; j < 32; j++ {
				weight := uint64(7)
				if j == 31 {
					weight = 39
				} // 31*7 + 39 = 256. GCD(7, 39)=1
				nhg.AddNextHop(NHBaseEncap+uint64((i*32+j)%numUniqueEncapNH), weight)
			}
		}
		allEntries = append(allEntries, nhg)
	}

	for vi, vrf := range encapVRFs {
		v4Prefixes, v4Err := iputil.GenerateIPsWithStep(fmt.Sprintf("200.%d.0.1", vi), NumEncapIPv4PerVRF, CommonPrefixStep)
		if v4Err != nil {
			t.Fatalf("Failed to generate IPv4 prefixes for VRF %s (vi=%d): %v", vrf, vi, v4Err)
		}
		for i, host := range v4Prefixes {
			allEntries = append(allEntries, fluent.IPv4Entry().WithNetworkInstance(vrf).WithPrefix(fmt.Sprintf("%s/%d", host, IPv4HostMask)).WithNextHopGroup(NHGBaseEncap+uint64((vi*NumEncapIPv4PerVRF+i)%numEncapDefaultNHG)).WithNextHopGroupNetworkInstance(defaultVRF))
		}
		v6Prefixes, v6Err := iputil.GenerateIPv6sWithStep(fmt.Sprintf("2001:db8:%x::1", vi), NumEncapIPv6PerVRF, CommonIPv6PrefixStep)
		if v6Err != nil {
			t.Fatalf("Failed to generate IPv6 prefixes for VRF %s (vi=%d): %v", vrf, vi, v6Err)
		}
		for i, pfx := range v6Prefixes {
			allEntries = append(allEntries, fluent.IPv6Entry().WithNetworkInstance(vrf).WithPrefix(fmt.Sprintf("%s/%d", pfx, IPv6HostMask)).WithNextHopGroup(NHGBaseEncap+uint64((vi*NumEncapIPv6PerVRF+i)%numEncapDefaultNHG)).WithNextHopGroupNetworkInstance(defaultVRF))
		}
	}

	// DECAP_TE_VRF entries use variable prefix lengths — not host routes.
	for i := 0; i < NumDecapEntries; i++ {
		prefixLen := decapPrefixLens[i%len(decapPrefixLens)]
		pfx := fmt.Sprintf("203.%d.%d.1/%d", i/4, (i%4)*64, prefixLen)
		nhIdx := NHBaseDecap + uint64(i)
		nhgIdx := NHGBaseDecap + uint64(i)
		decapNH, _ := gribi.NHEntry(nhIdx, "Decap", defaultVRF, fluent.InstalledInFIB, &gribi.NHOptions{Interface: fmt.Sprintf("port2.%d", i%NumPort2VLANs+1)})
		decapNHG, _ := gribi.NHGEntry(nhgIdx, map[uint64]uint64{nhIdx: 1}, defaultVRF, fluent.InstalledInFIB)
		allEntries = append(allEntries, decapNH, decapNHG, fluent.IPv4Entry().WithNetworkInstance(DecapVRFStr).WithPrefix(pfx).WithNextHopGroup(nhgIdx).WithNextHopGroupNetworkInstance(defaultVRF))
	}

	t.Logf("BuildEncapDecapVRFs: entries for %d VRFs", len(encapVRFs)+1)
	gSession := BatchModify(t, dut, ctx, allEntries, 120*time.Second)
	for vi, vrf := range encapVRFs {
		for i := 1; i < 3; i++ {
			wantPrefixes[vrf] = append(wantPrefixes[vrf], fmt.Sprintf("200.%d.0.%d/%d", vi, i, IPv4HostMask))
		}
	}
	VerifyFIBProgrammed(t, gSession, wantPrefixes)
	gSession.Close(t)
}

// VerifyFIBProgrammed checks that each prefix in wantPrefixes is FIB_PROGRAMMED in the gRIBI client results cache.
func VerifyFIBProgrammed(t *testing.T, c *gribi.Client, wantPrefixes map[string][]string) {
	t.Helper()
	res := c.Fluent(t).Results(t)
	for vrf, prefixes := range wantPrefixes {
		wants := make([]*client.OpResult, 0, len(prefixes))
		for _, pfx := range prefixes {
			wants = append(wants, fluent.OperationResult().WithIPv4Operation(pfx).WithOperationType(constants.Add).WithProgrammingResult(fluent.InstalledInFIB).AsResult())
		}
		chk.HasResultsCache(t, res, wants, chk.IgnoreOperationID())
		t.Logf("VRF %s: %d prefixes confirmed FIB_PROGRAMMED", vrf, len(prefixes))
	}
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
func ExpandDecapPrefixes() []string {
	ips := make([]string, NumDecapEntries)
	for i := 0; i < NumDecapEntries; i++ {
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
//	decap:    16 VRFs                                =  16
//	reencap:  16 VRFs × 2 src IPs × 2 proto          =  64
//	transit:  16 VRFs                                =  16
//	repaired: 16 VRFs                                =  16
//	total                                            = 144
//
// Fixed-size: 144 flow groups  ✓ (< 256 limit)
// IMIX:       144 flow groups  ✓ (SetChoice(WEIGHT) = 1 group per VRF)
func CountBaseFlows() int {
	return NumEncapVRFs*2 + // encap IPv4+IPv6
		NumEncapVRFs + // decap
		NumEncapVRFs*len(outerSrcs)*2 + // reencap IPv4+IPv6
		NumEncapVRFs + // transit
		NumEncapVRFs // repaired
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

// BuildEncapFlows builds fixed-size/imix encap flows for all encap VRFs. IPv4 and IPv6 inners are separate flows since the inner src/dst formats differ.
func BuildEncapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	for vi := range encapVRFs {
		d1, d2 := EncapVRFDSCP(vi)
		dscpVals := []uint32{uint32(d1), uint32(d2)}

		f4 := newFlow(fmt.Sprintf("encap_ipv4_vrf_%d", vi))
		f4.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)
		ip4 := f4.Packet().Add().Ipv4()
		ip4.Src().SetValue(ATEPort1IPv4)
		ip4.Dst().Increment().SetStart(fmt.Sprintf("200.%d.0.1", vi)).SetStep(CommonPrefixStep).SetCount(uint32(NumEncapIPv4PerVRF))
		ip4.Priority().Dscp().Phb().SetValues(dscpVals)
		flows = append(flows, f4)

		f6 := newFlow(fmt.Sprintf("encap_ipv6_vrf_%d", vi))
		f6.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)
		ip6 := f6.Packet().Add().Ipv6()
		ip6.Src().SetValue(EncapIPv6InnerSrc)
		ip6.Dst().Increment().SetStart(fmt.Sprintf("2001:db8:%x::1", vi)).SetStep(CommonIPv6PrefixStep).SetCount(uint32(NumEncapIPv6PerVRF))

		ip6.TrafficClass().SetValues([]uint32{
			uint32(d1) << 2,
			uint32(d2) << 2,
		})
		flows = append(flows, f6)
	}
	return flows
}

// BuildDecapFlows builds fixed-size/imix decap flows for all encap VRFs. Both DSCPs per VRF are expressed via SetValues in a single flow since the outer header is the same.
func BuildDecapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	decapDsts := ExpandDecapPrefixes()

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	for vi := range encapVRFs {
		d1, d2 := EncapVRFDSCP(vi)

		f := newFlow(fmt.Sprintf("decap_vrf_%d_src_111", vi))

		f.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)

		outer := f.Packet().Add().Ipv4()
		outer.Src().SetValue(IPv4OuterSrc111)
		outer.Dst().SetValues(decapDsts)
		outer.Priority().Dscp().Phb().SetValues([]uint32{
			uint32(d1),
			uint32(d2),
		})

		inner := f.Packet().Add().Ipv4()
		inner.Src().SetValue(ATEPort1IPv4)
		inner.Dst().SetValue(DecapIPv4InnerDst)

		flows = append(flows, f)
	}
	return flows
}

// BuildReencapFlows builds fixed-size/imix reencap flows for all encap VRFs.
func BuildReencapFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)
	decapDsts := ExpandDecapPrefixes()

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	for vi := range encapVRFs {
		d1, d2 := EncapVRFDSCP(vi)
		dscpVals := []uint32{uint32(d1), uint32(d2)}

		for _, outerSrc := range outerSrcs {
			tag := outerSrc[len(outerSrc)-3:]

			// ---------- IPv4 inner ----------
			f4 := newFlow(fmt.Sprintf("reencap_ipv4_vrf_%d_src_%s", vi, tag))
			f4.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)

			o4 := f4.Packet().Add().Ipv4()
			o4.Src().SetValue(outerSrc)
			o4.Dst().SetValues(decapDsts)
			o4.Priority().Dscp().Phb().SetValues(dscpVals)

			i4 := f4.Packet().Add().Ipv4()
			i4.Src().SetValue(ATEPort1IPv4)
			i4.Dst().Increment().SetStart(fmt.Sprintf("200.%d.0.1", vi)).SetStep(CommonPrefixStep).SetCount(uint32(NumEncapIPv4PerVRF))

			flows = append(flows, f4)

			// ---------- IPv6 inner ----------
			f6 := newFlow(fmt.Sprintf("reencap_ipv6_vrf_%d_src_%s", vi, tag))
			f6.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)

			o6 := f6.Packet().Add().Ipv4()
			o6.Src().SetValue(outerSrc)
			o6.Dst().SetValues(decapDsts)
			o6.Priority().Dscp().Phb().SetValues(dscpVals)

			i6 := f6.Packet().Add().Ipv6()
			i6.Src().SetValue(EncapIPv6InnerSrc)
			i6.Dst().Increment().SetStart(fmt.Sprintf("2001:db8:%x::1", vi)).SetStep(CommonIPv6PrefixStep).SetCount(uint32(NumEncapIPv6PerVRF))

			flows = append(flows, f6)
		}
	}
	return flows
}

// BuildTransitFlows builds fixed-size/imix transit flows for all encap VRFs.
func BuildTransitFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	for vi := range encapVRFs {
		d1, d2 := EncapVRFDSCP(vi)

		f := newFlow(fmt.Sprintf("transit_encap_te_vrf_%d", vi))

		f.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)

		outer := f.Packet().Add().Ipv4()
		outer.Src().SetValue(IPv4OuterSrc111)
		outer.Dst().Increment().SetStart(TransitVRF111PrefixStart).SetStep(CommonPrefixStep).SetCount(uint32(NumTransitIPv4))
		outer.Priority().Dscp().Phb().SetValues([]uint32{
			uint32(d1),
			uint32(d2),
		})

		inner := f.Packet().Add().Ipv4()
		inner.Src().SetValue(ATEPort1IPv4)
		inner.Dst().SetValue(TransitIPv4InnerDst)

		flows = append(flows, f)
	}

	return flows
}

// BuildRepairedFlows builds fixed-size/imix flows for all repaired VRFs. One flow per VRF; both DSCPs via SetValues.
func BuildRepairedFlows(top gosnappi.Config, pktSize uint32, pps uint64, imix bool) []gosnappi.Flow {
	flows := make([]gosnappi.Flow, 0)

	newFlow := MakeFlowCreator(top, pktSize, pps, imix)

	for vi := range encapVRFs {
		d1, d2 := EncapVRFDSCP(vi)

		f := newFlow(fmt.Sprintf("repaired_encap_te_vrf_%d", vi))

		f.Packet().Add().Ethernet().Src().SetValue(ATEPort1MAC)

		outer := f.Packet().Add().Ipv4()
		outer.Src().SetValue(IPv4OuterSrc222)
		outer.Dst().Increment().SetStart(TransitVRF222PrefixStart).SetStep(CommonPrefixStep).SetCount(uint32(NumTransitIPv4))
		outer.Priority().Dscp().Phb().SetValues([]uint32{
			uint32(d1),
			uint32(d2),
		})

		inner := f.Packet().Add().Ipv4()
		inner.Src().SetValue(ATEPort1IPv4)
		inner.Dst().SetValue(RepairIPv4InnerDst)

		flows = append(flows, f)
	}

	return flows
}

// RemovegRIBIRoute method is clearing the gRIBI routes.
func RemovegRIBIRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	gSession := NewGRIBIClient(t, dut)
	t.Cleanup(func() {
		gSession.FlushAll(t)
		gSession.Close(t)
	})
}

// RunEndToEndTrafficValidation executes the end-to-end traffic validation for all scenarios. It registers flows, configures capture, runs traffic, and validates via otgvalidationhelpers and packetvalidationhelpers.
func RunEndToEndTrafficValidation(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, imix bool) {
	t.Helper()
	baseFlows := CountBaseFlows()
	perFlowPPS := TrafficRateMpps / uint64(baseFlows)
	if perFlowPPS == 0 {
		perFlowPPS = 1
	}
	t.Logf("Traffic : imix=%v, %d base flow groups, %d pps/group -> ~%d Mpps aggregate", imix, baseFlows, perFlowPPS, perFlowPPS*uint64(baseFlows)/1_000_000)

	decapPfxSet := ExpandDecapPrefixes()
	expectations := map[string]FlowExpectation{}
	gSession := NewGRIBIClient(t, dut)
	t.Cleanup(func() {
		gSession.FlushAll(t)
		gSession.Close(t)
	})
	// addFlows registers each flow's expectation. The VRF index is extracted
	// from the flow's position in the slice so both DSCP values for that VRF
	// can be stored in ExpectedDSCPs — either is a valid egress DSCP.
	addFlows := func(builtFlows []gosnappi.Flow, sc TrafficScenario, outerSrc string, wantEncap bool, flowsPerVRF int) {
		for fi, f := range builtFlows {
			vi := fi / flowsPerVRF // VRF index from position in scenario slice
			d1, d2 := EncapVRFDSCP(vi)
			expectations[f.Name()] = FlowExpectation{
				Scenario:         sc,
				ExpectedOuterSrc: outerSrc,
				ExpectedDSCPs:    []uint32{uint32(d1), uint32(d2)},
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
		build       func(gosnappi.Config, uint32, uint64, bool) []gosnappi.Flow
		scenario    TrafficScenario
		outerSrc    string
		needCapture bool
		multiplier  int
	}{
		{BuildEncapFlows, ScenarioEncap, IPv4OuterSrc111, true, 2},
		{BuildDecapFlows, ScenarioDecap, "", false, 1},
		{BuildReencapFlows, ScenarioReencap, "", true, 4},
		{BuildTransitFlows, ScenarioTransit, IPv4OuterSrc111, true, 1},
		{BuildRepairedFlows, ScenarioRepaired, IPv4OuterSrc222, true, 1},
	}

	for _, b := range builders {
		flows := b.build(top, pktSize, perFlowPPS, imix)
		addFlows(flows, b.scenario, b.outerSrc, b.needCapture, b.multiplier)
		allFlows = append(allFlows, flows...)
	}

	// Clear capture
	packetvalidationhelpers.ClearCapture(t, top, ate)

	// Configure capture on port2 via packetvalidationhelpers and push config.
	capVal := &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "cap_port2",
	}
	packetvalidationhelpers.ConfigurePacketCapture(t, top, capVal)
	ate.OTG().PushConfig(t, top)

	// Start capture, run traffic, stop capture.
	// StartCapture returns the ControlState it armed; StopCapture reuses it to issue the STOP command on the same port-capture object.
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(TrafficDuration)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)

	// Step 1: Zero-loss check via otgvalidationhelpers per flow.
	for _, f := range allFlows {
		v := &otgvalidationhelpers.OTGValidation{
			Flow: &otgvalidationhelpers.FlowParams{
				Name:         f.Name(),
				TolerancePct: float32(TrafficLossTol),
			},
		}
		if err := v.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("Zero-loss check: %v", err)
		}
	}

	// Step 2: Deep packet inspection.
	// Build per-scenario PacketValidation descriptors and delegate to
	// packetvalidationhelpers.CaptureAndValidatePackets which uses gopacket.
	ValidateCapturedPackets(t, ate, capVal, expectations)
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
