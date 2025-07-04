package hashing

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
	transitMagicSrcIP  = "111.111.111.111"
	repairedMagicSrcIP = "222.222.222.222"
)

// Traffic flow variables.
var (
	loadBalancingTolerance = 0.03
	rSiteV4DSTIP           = "10.240.118.50"
	eSiteV4DSTIP           = "10.240.118.35"
	rSiteV6DSTIP           = "2002:af0:7730:a::1"
	eSiteV6DSTIP           = "2002:af0:7620:e::1"
	v4TrafficType          = "ipv4"
	v6TrafficType          = "ipv6"
	noTrafficType          = ""
	srcIPFlowCount         = 50000
	L4FlowCount            = 50000
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
		TrafficPPS:        200,
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
		TrafficPPS:        200,
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
		OuterDSCP:          104,
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
		OuterDSCP:          104,
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
		OuterSrcStart:     transitMagicSrcIP,
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
		TrafficPPS:        200,
		PacketSize:        128,
	}
	IPinIPE2R = helper.TrafficFlowAttr{
		FlowName:          "IPinIPE2R",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     transitMagicSrcIP,
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
		TrafficPPS:        200,
		PacketSize:        128,
	}
)

type extendedEntropyCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

type algorithmAdjustCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

type testCase struct {
	name                  string
	desc                  string
	extendedEntropyOption *extendedEntropyCLIOptions
	algorithmAdjustOption *algorithmAdjustCLIOptions
	confHashCLIdutList    []*ondatra.DUTDevice
}

type BundleInterface struct {
	BundleInterfaceName string
	BundleNHWeight      uint64
	BundleMembers       []string
	BundleMembersWeight []uint64
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

// func programGribiEntries(t *testing.T, dut *ondatra.DUTDevice, inputGribiEntries []helper.GribiLISPEntry) {
// 	// Construct the GRIBI LISP entry payload
// 	var gribiEntries []helper.GribiLISPEntry
// 	for _, entry := range inputGribiEntries {
// 		gribiEntry := helper.GribiLISPEntry{
// 			Instance: entry.Instance,
// 			Protocol: entry.Protocol,
// 			Source:   entry.Source,
// 			EID:      entry.EID,
// 			Locator:  entry.Locator,
// 		}
// 		gribiEntries = append(gribiEntries, gribiEntry)
// 	}
// }
