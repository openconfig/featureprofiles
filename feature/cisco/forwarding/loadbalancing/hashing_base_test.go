package hashing

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	cardTypeRp         = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	ipv4PrefixLen      = 30
	v4PrefixLen32      = 32
	ipv6PrefixLen      = 126
	v432PfxLen         = "/32"
	transitMagicSrcIP  = "111.111.111.111"
	repairedMagicSrcIP = "222.222.222.222"
	vrfEncapA          = "VRF-HighPriority"
	vrfEncapB          = "VRF-LowPriority"
	vrfTransit         = "TRANSIT_VRF"
	vrfRepaired        = "REPAIRED"
	vrfRepair          = "REPAIR"
	vrfDecap           = "DECAP_TE_VRF"
	localStationMac    = "00:1a:11:17:5f:80"
	trafficRatePPS 	   = 10000
)

// Traffic flow and common variables.
var (
	loadBalancingTolerance = 0.03
	rSiteV4DSTIP           = "10.240.118.50"
	eSiteV4DSTIP           = "10.240.118.35"
	rSiteV6DSTIP           = "2002:af0:7730:a::1"
	rSiteV6DSTPFX          = "2002:af0:7730:a::"
	eSiteV6DSTIP           = "2002:af0:7620:e::1"
	eSiteV6DSTPFX          = "2002:af0:7620:e::"
	noTrafficType          = ""
	srcIPFlowCount         = 50000
	L4FlowCount            = 50000
	useOTG                 = false
	nextSiteVIPWeight      = uint64(1)
	selfSiteVIPWeight      = uint64(1)
)

// DUT and TGEN port attributes
var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "dutPort1",
		MAC:     "00:aa:00:bb:00:cc",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		Name:    "port2",
		MAC:     "00:bb:00:11:00:dd",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "04:00:02:02:02:02",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}
)

// Routed Traffic flow attributes using TrafficFlowAttr struct.
var (
	//V4 Routed Traffic flows
	v4R2E = helper.TrafficFlowAttr{
		FlowName:          "IPv4R2E",
		DstMacAddress:     dutPort1.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "10.240.213.42",
		OuterDstStart:     eSiteV4DSTIP,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: uint32(srcIPFlowCount),
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		TgenSrcPort:       atePort1,
		TgenDstPorts:      []string{atePort2.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4RandomMin:       1000,
		L4RandomMax:       65000,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        trafficRatePPS,
		PacketSize:        128,
	}
	v4E2R = helper.TrafficFlowAttr{
		FlowName:          "IPv4E2R",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "10.240.113.42",
		OuterDstStart:     rSiteV4DSTIP,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: uint32(srcIPFlowCount),
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		TgenSrcPort:       atePort2,
		TgenDstPorts:      []string{atePort1.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4RandomMin:       1000,
		L4RandomMax:       65000,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        trafficRatePPS,
		PacketSize:        128,
	}

	//V6 Routed Traffic flows
	v6R2E = helper.TrafficFlowAttr{
		FlowName:           "IPv6R2E",
		DstMacAddress:      dutPort1.MAC,
		OuterProtocolType:  "IPv6",
		OuterSrcStart:      "2002:af0:a000::",
		OuterDstStart:      eSiteV6DSTIP,
		OuterSrcStep:       "::1",
		OuterSrcFlowCount:  uint32(srcIPFlowCount),
		OuterDstFlowCount:  1,
		OuterDstStep:       "::1",
		OuterDSCP:          10,
		OuterTTL:           155,
		OuterECN:           1,
		OuterIPv6Flowlabel: 1000,
		TgenSrcPort:        atePort1,
		TgenDstPorts:       []string{atePort2.Name},
		L4TCP:              true,
		L4PortRandom:       true,
		L4RandomMin:        1000,
		L4RandomMax:        65000,
		L4SrcPortStart:     1000,
		L4DstPortStart:     2000,
		L4FlowStep:         1,
		L4FlowCount:        uint32(srcIPFlowCount),
		TrafficPPS:         200,
		PacketSize:         128,
	}
	v6E2R = helper.TrafficFlowAttr{
		FlowName:           "IPv6E2R",
		DstMacAddress:      dutPort2.MAC,
		OuterProtocolType:  "IPv6",
		OuterSrcStart:      "2002:af0:a000::",
		OuterDstStart:      rSiteV6DSTIP,
		OuterSrcStep:       "::1",
		OuterSrcFlowCount:  uint32(srcIPFlowCount),
		OuterDstFlowCount:  1,
		OuterDstStep:       "::1",
		OuterDSCP:          10,
		OuterTTL:           155,
		OuterECN:           1,
		OuterIPv6Flowlabel: 1000,
		TgenSrcPort:        atePort2,
		TgenDstPorts:       []string{atePort1.Name},
		L4TCP:              true,
		L4PortRandom:       true,
		L4RandomMin:        1000,
		L4RandomMax:        65000,
		L4SrcPortStart:     1000,
		L4DstPortStart:     2000,
		L4FlowStep:         1,
		L4FlowCount:        uint32(srcIPFlowCount),
		TrafficPPS:         200,
		PacketSize:         128,
	}
	// IPinIP Traffic flows
	IPinIPR2E = helper.TrafficFlowAttr{
		FlowName:          "IPinIPR2E",
		DstMacAddress:     dutPort1.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "111.111.111.1",
		OuterDstStart:     eSiteV4DSTIP,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 1,
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		InnerProtocolType: "IPv4",
		InnerSrcStart:     "10.240.213.42",
		InnerDstStart:     "98.2.0.1",
		InnerSrcStep:      "0.0.0.1",
		InnerSrcFlowCount: uint32(srcIPFlowCount),
		InnerDstFlowCount: 1,
		InnerDstStep:      "0.0.0.1",
		InnerDSCP:         10,
		InnerTTL:          55,
		InnerECN:          1,
		TgenSrcPort:       atePort1,
		TgenDstPorts:      []string{atePort2.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4RandomMin:       1000,
		L4RandomMax:       65000,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        trafficRatePPS,
		PacketSize:        128,
	}
	IPinIPE2R = helper.TrafficFlowAttr{
		FlowName:          "IPinIPE2R",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "111.111.111.1",
		OuterDstStart:     rSiteV4DSTIP,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 1,
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		InnerProtocolType: "IPv4",
		InnerSrcStart:     "10.240.113.42",
		InnerDstStart:     "98.2.0.1",
		InnerSrcStep:      "0.0.0.1",
		InnerSrcFlowCount: uint32(srcIPFlowCount),
		InnerDstFlowCount: 1,
		InnerDstStep:      "0.0.0.1",
		InnerDSCP:         10,
		InnerTTL:          55,
		InnerECN:          1,
		TgenSrcPort:       atePort2,
		TgenDstPorts:      []string{atePort1.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4RandomMin:       1000,
		L4RandomMax:       65000,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        trafficRatePPS,
		PacketSize:        128,
	}
	// IPv6inIP Traffic flows
	IPv6inIPR2E = helper.TrafficFlowAttr{
		FlowName:           "IPv6inIPR2E",
		DstMacAddress:      dutPort1.MAC,
		OuterProtocolType:  "IPv4",
		OuterSrcStart:      "111.111.111.1",
		OuterDstStart:      eSiteV4DSTIP,
		OuterSrcStep:       "0.0.0.1",
		OuterSrcFlowCount:  1,
		OuterDstFlowCount:  1,
		OuterDstStep:       "0.0.0.1",
		OuterDSCP:          10,
		OuterTTL:           55,
		OuterECN:           1,
		InnerProtocolType:  "IPv6",
		InnerSrcStart:      "2002:af0:a000::",
		InnerDstStart:      eSiteV6DSTIP,
		InnerSrcStep:       "::1",
		InnerSrcFlowCount:  uint32(srcIPFlowCount),
		InnerDstFlowCount:  1,
		InnerDstStep:       "::1",
		InnerDSCP:          104,
		InnerTTL:           155,
		InnerECN:           1,
		InnerIPv6Flowlabel: 1000,
		TgenSrcPort:        atePort1,
		TgenDstPorts:       []string{atePort2.Name},
		L4TCP:              true,
		L4PortRandom:       true,
		L4RandomMin:        1000,
		L4RandomMax:        65000,
		L4SrcPortStart:     1000,
		L4DstPortStart:     2000,
		L4FlowStep:         1,
		L4FlowCount:        uint32(srcIPFlowCount),
		TrafficPPS:         200,
		PacketSize:         128,
	}
	IPv6inIPE2R = helper.TrafficFlowAttr{
		FlowName:          "IPv6inIPE2R",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "111.111.111.1",

		OuterDstStart:      rSiteV4DSTIP,
		OuterSrcStep:       "0.0.0.1",
		OuterSrcFlowCount:  1,
		OuterDstFlowCount:  1,
		OuterDstStep:       "0.0.0.1",
		OuterDSCP:          10,
		OuterTTL:           55,
		OuterECN:           1,
		InnerProtocolType:  "IPv6",
		InnerSrcStart:      "2002:af0:a000::",
		InnerDstStart:      rSiteV6DSTIP,
		InnerSrcStep:       "::1",
		InnerSrcFlowCount:  uint32(srcIPFlowCount),
		InnerDstFlowCount:  1,
		InnerDstStep:       "::1",
		InnerDSCP:          104,
		InnerTTL:           155,
		InnerECN:           1,
		InnerIPv6Flowlabel: 1000,
		TgenSrcPort:        atePort2,
		TgenDstPorts:       []string{atePort1.Name},
		L4TCP:              true,
		L4PortRandom:       true,
		L4RandomMin:        1000,
		L4RandomMax:        65000,
		L4SrcPortStart:     1000,
		L4DstPortStart:     2000,
		L4FlowStep:         1,
		L4FlowCount:        uint32(srcIPFlowCount),
		TrafficPPS:         200,
		PacketSize:         128,
	}
)

// CLI options for configuring extended entropy CLI options.
type extendedEntropyCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

// CLI options for configuring algorithm adjust CLI options.
type algorithmAdjustCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

// Test case struct for testcase args.
type testCase struct {
	name                  string
	desc                  string
	extendedEntropyOption *extendedEntropyCLIOptions
	algorithmAdjustOption *algorithmAdjustCLIOptions
	confHashCLIdutList    []*ondatra.DUTDevice
}

// Bundle interface struct for bundle interface name, bundle members and their respective weights.
type BundleInterface struct {
	BundleInterfaceName string
	BundleNHWeight      uint64
	BundleMembers       []string
	BundleMembersWeight []uint64
}

type gribiParamPerSite struct {
	dut               *ondatra.DUTDevice
	encapV4Prefix     string              // Encap VRF Prefix IP list with prefix length
	encapV6Prefix     string              // Encap VRF Prefix IP list with prefix length
	encapTunnelIP1    string              // Encap VRF Tunnel IP.
	encapTunnelIP2    string              // Encap VRF Tunnel IP.
	decapV4Prefix     string              // Decap VRF Prefix IP list with prefix length
	decapV6Prefix     string              // Decap VRF Prefix IP list with prefix length
	nextSiteVIPs      []string            // Next site VIP Prefix IP list with prefix length
	selfSiteVIPs      []string            // Self site VIP Prefix IP list  prefix length
	nextSiteIntfCount int                 // Next site VIP Next Hop Interface count
	selfSiteIntfCount int                 // Self site VIP Next Hop Interface count
	nextSite1VIPNH    []map[string]string // Next site VIP Next Hop info with map with key as interface name and value as next hop IP
	nextSite2VIPNH    []map[string]string // Self site VIP Next Hop info with map with key as interface name and value as next hop IP
	selfSiteVIPNH     []map[string]string // Self site VIP Next Hop Interface list
}

func configureHashCLIOptions(t *testing.T, extendHash *extendedEntropyCLIOptions, hashRotate *algorithmAdjustCLIOptions, dutList []*ondatra.DUTDevice, delete bool) {
	for _, dut := range dutList {
		if extendHash != nil {
			if delete {
				// Delete extended entropy configuration for all options
				t.Log("Deleting extended entropy configuration")
				config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy auto-global\n no cef platform load-balancing extended-entropy auto-instance\n no cef platform load-balancing extended-entropy profile-index")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy auto-instance")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy profile-index")
			} else {
				t.Log("Configuring extended entropy configuration")
				if extendHash.perChassis {
					// Configure for per chassis
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-global")
				}
				if extendHash.perNPU {
					// Configure for per NPU
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-instance")
				}
				if extendHash.specificVal != 0 {
					// Configure for specific value
					config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing extended-entropy profile-index %d", extendHash.specificVal))
				}
			}
		}
		if hashRotate != nil {
			if delete {
				// Delete hash rotation configuration for all options
				t.Log("Deleting hash rotate configuration")
				config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust auto-global\n no cef platform load-balancing algorithm adjust auto-instance\n no cef platform load-balancing algorithm adjust")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust auto-instance")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust")
			} else {
				t.Log("Configuring hash rotate configuration")
				if hashRotate.perChassis {
					// Configure for per chassis
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-global")
				}
				if hashRotate.perNPU {
					// Configure for per NPU
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-instance")
				}
				if hashRotate.specificVal != 0 {
					// Configure for specific value
					config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing algorithm adjust %d", hashRotate.specificVal))
				}
			}
		}
	}
}

func programGribiEntries(t *testing.T, dut *ondatra.DUTDevice, gribiArgs gribiParamPerSite) {
	// Configure the gRIBI client
	gribiClient := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	gribiClient.BecomeLeader(t)
	gribiClient.FlushAll(t)
	// defer gribiClient.FlushAll(t)
	t.Logf("Adding %s VRF gRIBI entries", vrfEncapA)
	//Backup NHG with redirect/NH to default VRF
	gribiClient.AddNH(t, 1104, "VRFOnly", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(dut)})
	gribiClient.AddNHG(t, 335548321, map[uint64]uint64{1104: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 335548300, map[uint64]uint64{1104: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//gRIBI entries for Encap VRF prefixes
	gribiClient.AddNH(t, 2342, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: transitMagicSrcIP, Dest: gribiArgs.encapTunnelIP1, VrfName: vrfTransit})
	gribiClient.AddNH(t, 2334, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: transitMagicSrcIP, Dest: gribiArgs.encapTunnelIP2, VrfName: vrfTransit})
	gribiClient.AddNHG(t, 335544321, map[uint64]uint64{2342: 1, 2334: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 335548321})
	gribiClient.AddIPv4(t, gribiArgs.encapV4Prefix, 335544321, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//Add default route for Encap VRF
	gribiClient.AddIPv4(t, "0.0.0.0/0", 335548300, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv6(t, gribiArgs.encapV6Prefix, 335544321, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	t.Logf("Adding %s gRIBI entries for %s", vrfTransit, gribiArgs.encapTunnelIP1)
	nextSiteNHGtMap := make(map[uint64]uint64)
	//Add Next Hop Group for Next Site VIPs
	for i, nhInfo := range gribiArgs.nextSite1VIPNH {
		for nhIntf, nhIP := range nhInfo {
			gribiClient.AddNH(t, 2350+uint64(i), "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: nhIP, Mac: localStationMac, Interface: nhIntf})
			nextSiteNHGtMap[2350+uint64(i)] = 1
		}
	}
	gribiClient.AddNHG(t, 402653185, nextSiteNHGtMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//Add Next Hop Group for Self Site VIPs
	selfSiteNHGtMap := make(map[uint64]uint64)
	for i, nhInfo := range gribiArgs.selfSiteVIPNH {
		for nhIntf, nhIP := range nhInfo {
			gribiClient.AddNH(t, 2450+uint64(i), "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: nhIP, Mac: localStationMac, Interface: nhIntf})
			selfSiteNHGtMap[2450+uint64(i)] = 1
		}
	}
	gribiClient.AddNHG(t, 402653186, selfSiteNHGtMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 2331, gribiArgs.nextSiteVIPs[0], deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNH(t, 2332, gribiArgs.selfSiteVIPs[0], deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//Add Backup NHG with redirect to Repair VRF.
	gribiClient.AddNH(t, 2, "VRFOnly", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfRepair})
	gribiClient.AddNHG(t, 2, map[uint64]uint64{2: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	if gribiArgs.selfSiteIntfCount == 0 {
		gribiClient.AddNHG(t, 402653184, map[uint64]uint64{2331: nextSiteVIPWeight}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 2})

	} else {
		gribiClient.AddNHG(t, 402653184, map[uint64]uint64{2331: nextSiteVIPWeight, 2332: selfSiteVIPWeight}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 2})

	}
	gribiClient.AddIPv4(t, gribiArgs.encapTunnelIP1+v432PfxLen, 402653184, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, gribiArgs.nextSiteVIPs[0]+v432PfxLen, 402653185, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, gribiArgs.selfSiteVIPs[0]+v432PfxLen, 402653186, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	if gribiArgs.nextSiteIntfCount > 1 {
		t.Logf("Adding %s gRIBI entries for %s", vrfTransit, gribiArgs.encapTunnelIP2)
		nextSiteNHGtMap = make(map[uint64]uint64)
		//Add Next Hop Group for Next Site VIPs
		for i, nhInfo := range gribiArgs.nextSite2VIPNH {
			for nhIntf, nhIP := range nhInfo {
				gribiClient.AddNH(t, 5350+uint64(i), "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: nhIP, Mac: localStationMac, Interface: nhIntf})
				nextSiteNHGtMap[5350+uint64(i)] = 1
			}
		}
		gribiClient.AddNHG(t, 502653185, nextSiteNHGtMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		//Add Next Hop Group for Self Site VIPs
		// selfSiteNHGtMap = make(map[uint64]uint64)
		// for i, nhInfo := range gribiArgs.selfSiteVIPNH {
		// 	for nhIntf, nhIP := range nhInfo {
		// 		gribiClient.AddNH(t, 5450+uint64(i), "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: nhIP, Mac: localStationMac, Interface: nhIntf})
		// 		selfSiteNHGtMap[5450+uint64(i)] = 1
		// 	}
		// }
		// gribiClient.AddNHG(t, 502653186, selfSiteNHGtMap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		gribiClient.AddNH(t, 5331, gribiArgs.nextSiteVIPs[1], deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		// gribiClient.AddNH(t, 5332, gribiArgs.selfSiteVIPs[0], deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		//Add Backup NHG with redirect to Repair VRF.
		gribiClient.AddNHG(t, 502653184, map[uint64]uint64{5331: nextSiteVIPWeight}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 2})
		gribiClient.AddIPv4(t, gribiArgs.encapTunnelIP2+v432PfxLen, 502653184, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		gribiClient.AddIPv4(t, gribiArgs.nextSiteVIPs[1]+v432PfxLen, 502653185, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		// gribiClient.AddIPv4(t, gribiArgs.selfSiteVIPs[0]+v432PfxLen, 502653186, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	}

	t.Logf("Adding %s VRF gRIBI entries", vrfRepair)
	//Decap Backup NHG
	gribiClient.AddNH(t, 1, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 1, map[uint64]uint64{1: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	//encapTunnelIP1 repair VRF entries
	gribiClient.AddNH(t, 6000, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: repairedMagicSrcIP, Dest: gribiArgs.encapTunnelIP1, VrfName: vrfRepaired})
	gribiClient.AddNHG(t, 602653184, map[uint64]uint64{6000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 1})
	gribiClient.AddIPv4(t, gribiArgs.encapTunnelIP1+v432PfxLen, 602653184, vrfRepair, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	//encapTunnelIP2 repair VRF entries
	gribiClient.AddNH(t, 7000, "DecapEncap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: repairedMagicSrcIP, Dest: gribiArgs.encapTunnelIP1, VrfName: vrfRepaired})
	gribiClient.AddNHG(t, 702653184, map[uint64]uint64{7000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 1})
	gribiClient.AddIPv4(t, gribiArgs.encapTunnelIP1+v432PfxLen, 702653184, vrfRepair, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	t.Logf("Adding %s VRF gRIBI entries", vrfRepaired)
	gribiClient.AddNH(t, 8000, gribiArgs.nextSiteVIPs[0], deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 802653184, map[uint64]uint64{8000: 32}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: 2})
	gribiClient.AddIPv4(t, gribiArgs.encapTunnelIP1+v432PfxLen, 802653184, vrfRepaired, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	t.Logf("Adding %s VRF gRIBI entries", vrfDecap)
	gribiClient.AddNH(t, 1000, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddNHG(t, 335548320, map[uint64]uint64{1000: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.AddIPv4(t, gribiArgs.decapV4Prefix, 335548320, vrfDecap, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	gribiClient.Close(t)
}
