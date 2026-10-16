// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package basic_encap_test implements TE-16.1: gRIBI Base Hierarchical Traffic Encapsulation
// and Simultaneous IP-in-IP Tunnel and Subinterface Egress Streaming Telemetry Counters Validation.
package basic_encap_test

import (
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipipProtocol       = 4
	ipv6ipProtocol     = 41
	udpProtocol        = 17
	vrfDecap           = "DECAP_TE_VRF"
	vrfTransit         = "TE_VRF_111"
	vrfEncapA          = "ENCAP_TE_VRF_A"
	vrfEncapB          = "ENCAP_TE_VRF_B"
	ipv4PrefixLen      = 30
	ipv6PrefixLen      = 126
	trafficDuration    = 8 * time.Second
	defaultTrafficPPS  = uint64(1000)
	nhg10ID            = 10
	nh201ID            = 201
	nh202ID            = 202
	nhg1ID             = 1
	nh1ID              = 1
	nh2ID              = 2
	nhg2ID             = 2
	nh10ID             = 10
	nh11ID             = 11
	nhg3ID             = 3
	nh100ID            = 100
	nh101ID            = 101
	dscpEncapA1        = 10
	dscpEncapNoMatch   = 30
	magicIP            = "192.168.1.1"
	magicMac           = "02:00:00:00:00:01"
	tunnelDstIP1       = "203.0.113.1"
	tunnelDstIP2       = "203.0.113.2"
	ipv4OuterSrc111    = "198.51.100.111"
	ipv4OuterSrcIPInIP = "198.100.200.123"
	vipIP1             = "192.0.2.111"
	vipIP2             = "192.0.2.222"
	innerV4DstIP       = "198.18.1.1"
	innerV4SrcIP       = "198.18.0.255"
	InnerV6SrcIP       = "2001:DB8::198:1"
	InnerV6DstIP       = "2001:DB8:2:0:192::10"
	ipv4FlowIP         = "138.0.11.8"
	ipv4EntryPrefix    = "138.0.11.0"
	ipv4EntryPrefixLen = 24
	ipv6FlowIP         = "2015:aa8::1"
	ipv6EntryPrefix    = "2015:aa8::"
	ipv6EntryPrefixLen = 64
	nhg1000ID          = 1000
	nh1001ID           = 1001
	nhg2000ID          = 2000
	nh2001ID           = 2001
	nhg2002ID          = 2002
	nhg3000ID          = 3000
	nh3001ID           = 3001
	decapIPv4FlowIP    = "198.18.11.8"
	decapIPv4Prefix    = "198.18.11.0"
	decapIPv4PrefixLen = 24
	decapIPv6Prefix    = "2001:db8:2016::"
	decapIPv6PrefixLen = 64
	decapInnerDstIP4   = "192.0.2.100"
	decapInnerDstIP6   = "2001:db8::100"
	ratioTunEncap1     = 0.25 // 1/4
	ratioTunEncap2     = 0.75 // 3/4
	ratioTunEncapTol   = 0.05 // 5/100
	ttl                = uint32(100)
	trfDistTolerance   = 0.02
	// observing on IXIA OTG: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port in the chassis
	otgMutliPortCaptureSupported = false

	// Test-6 (Simultaneous IP-in-IP Encapsulation Tunnel and Subinterface Egress Streaming Telemetry Counters Validation) constants.
	nativeFallbackIPv6FlowIP = "1999:aa8::1"
	test6SampleInterval      = 10 * time.Second
	test6FrameSize           = uint32(512)
	test6InnerL3Bytes        = uint64(494) // 512B frame - 14B Ethernet header - 4B FCS
	test6InnerL2Bytes        = uint64(512) // 512B full L2 frame
	test6OuterL3Bytes        = uint64(514) // 494B inner IP packet + 20B outer IPv4 IP-in-IP header
	test6OuterL2Bytes        = uint64(532) // 512B inner frame + 20B outer IPv4 IP-in-IP header
	test6PPS                 = uint64(1000)
	test6PktCount            = uint32(35000) // 35s sustained transmission at 1,000 pps (> 3x 10s sample_interval)
	test6CiscoPktCount       = uint32(70000) // 70s sustained transmission at 1,000 pps (> 2x 30s Cisco statsd sync interval)
	test6MonotonicSamples    = 3
	test6SampleTimeout       = 45 * time.Second
	test6CounterTolPct       = 0.01 // +/- 1% packet/octet counter accuracy tolerance
)

var (
	otgDstPorts = []string{"port2", "port3", "port4", "port5"}
	otgSrcPort  = "port1"
	wantWeights = []float64{
		0.0625, // 1/4 * 1/4 - port1
		0.1875, // 1/4 * 3/4 - port2
		0.3,    // 3/4 * 2/5 - port3
		0.45,   // 3/5 * 3/4 - port4
	}
	noMatchWeight = []float64{
		1, 0, 0, 0,
	}
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort1 = attrs.Attributes{
		Name:    "otgPort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort2 = attrs.Attributes{
		Name:    "otgPort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort3 = attrs.Attributes{
		Name:    "otgPort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:D",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort4 = attrs.Attributes{
		Name:    "otgPort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:E",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:11",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort5 = attrs.Attributes{
		Name:    "otgPort5",
		MAC:     "02:00:05:01:01:01",
		IPv4:    "192.0.2.18",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:12",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3DummyIP = attrs.Attributes{
		Desc:       "dutPort3",
		IPv4Sec:    "192.0.2.25",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.26",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:       "dutPort4",
		IPv4Sec:    "192.0.2.29",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.30",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort5DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.33",
		IPv4LenSec: ipv4PrefixLen,
	}

	otgPort5DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.34",
		IPv4Len: ipv4PrefixLen,
	}
)

type EcnCode uint32

const (
	EcnNotECT EcnCode = 0 // 00: Non-ECN-capable transport
	EcnECT1   EcnCode = 1 // 01: ECT(1)
	EcnECT0   EcnCode = 2 // 10: ECT(0)
	EcnCE     EcnCode = 3 // 11: Congestion Experienced (CE)
)

var ecnNames = map[EcnCode]string{
	EcnNotECT: "Not-ECT(00)",
	EcnECT1:   "ECT(1)(01)",
	EcnECT0:   "ECT(0)(10)",
	EcnCE:     "CE(11)",
}

// priority packs 6-bit DSCP and 2-bit ECN into an 8-bit TOS/TrafficClass byte.
func priority(dscp uint32, ecn EcnCode) uint32 {
	return (dscp << 2) | uint32(ecn)
}

type packetAttr struct {
	dscp              int
	protocol          int
	ttl               uint32
	ecn               EcnCode
	validateECN       bool
	isDecap           bool
	isDecapV6         bool
	expectedEgressECN EcnCode
}

type flowAttr struct {
	src      string   // source IP address
	dst      string   // destination IP address
	srcPort  string   // source OTG port
	dstPorts []string // destination OTG ports
	srcMac   string   // source MAC address
	dstMac   string   // destination MAC address
	topo     gosnappi.Config
}

var (
	fa4 = flowAttr{
		src:      otgPort1.IPv4,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	fa6 = flowAttr{
		src:      otgPort1.IPv6,
		dst:      ipv6FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	faIPinIP = flowAttr{
		src:      ipv4OuterSrcIPInIP,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		topo:     gosnappi.NewConfig(),
	}
	faDecap = flowAttr{
		src:      ipv4OuterSrc111,
		dst:      decapIPv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: []string{"port2"},
		topo:     gosnappi.NewConfig(),
	}
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

var kneVirtualModels = []string{
	"ceos",         // Arista cEOS
	"xrd", "8000e", // Cisco XRD and 8000e
	"ncptx", "cptx", "vrx", // Juniper cPTX / ncptx / vRX
	"srlinux", "gen2cp", // Nokia SR Linux / Virtual Gen2 Control Plane
	"lemming", // OpenConfig Lemming
}

// isVirtualDUT reports whether the DUT is a virtual container/VM in KNE topologies.
func isVirtualDUT(dut *ondatra.DUTDevice) bool {
	// 1. Check KNE runtime flags (the most fundamental and reliable signal).
	// In WBB and Ondatra, virtual KNE runs explicitly pass --use_knegce, --use_kne_topo, or --topology.
	for _, fName := range []string{"use_knegce", "use_kne_topo", "topology", "kne-topo", "kne_topo"} {
		if f := flag.Lookup(fName); f != nil {
			val := f.Value.String()
			if val != "" && val != "false" && val != "0" {
				return true
			}
		}
	}

	// 2. Check virtual device models across all supported KNE vendors.
	model := strings.ToLower(dut.Model())
	for _, vm := range kneVirtualModels {
		if strings.Contains(model, vm) {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBasicEncap(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	// configure gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	defer c.Close(t)
	// flush all AFT entries after test
	defer c.FlushAll(t)
	c.BecomeLeader(t)

	// flush all existing AFT entries on the router
	c.FlushAll(t)

	programEntries(t, dut, &c)

	test := []struct {
		name               string
		pattr              packetAttr
		flows              []gosnappi.Flow
		weights            []float64
		capturePorts       []string
		validateEncapRatio bool
	}{
		// Test-1: IPv4 Traffic Encapsulation and WCMP Hierarchy Validation.
		{
			name:               fmt.Sprintf("Test1 IPv4 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		// Test-2: IPv6 Traffic Encapsulation and WCMP Hierarchy Validation.
		{
			name:               fmt.Sprintf("Test2 IPv6 Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99},
			flows:              []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		// Test-3: IP-in-IP (IPv4-in-IPv4 and IPv6-in-IPv4) Traffic Encapsulation and WCMP Hierarchy Validation.
		{
			name:  fmt.Sprintf("Test3 IPinIP Traffic WCMP Encap dscp %d", dscpEncapA1),
			pattr: packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99},
			flows: []gosnappi.Flow{faIPinIP.getFlow("ipv4in4", "ip4in4a1", dscpEncapA1),
				faIPinIP.getFlow("ipv6in4", "ip6in4a1", dscpEncapA1),
			},
			weights:            wantWeights,
			capturePorts:       otgDstPorts,
			validateEncapRatio: true,
		},
		// Test-4: Explicit Congestion Notification (ECN) Bits Preservation During IP-in-IP Encapsulation (IPv4 and IPv6).
		{
			name:               fmt.Sprintf("Test4 IPv4 Traffic Encap ECN %s dscp %d", ecnNames[EcnNotECT], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99, ecn: EcnNotECT, validateECN: true},
			flows:              []gosnappi.Flow{fa4.getFlowWithECN("ipv4", "ip4ecn_notect", dscpEncapA1, EcnNotECT)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv4 Traffic Encap ECN %s dscp %d", ecnNames[EcnECT1], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99, ecn: EcnECT1, validateECN: true},
			flows:              []gosnappi.Flow{fa4.getFlowWithECN("ipv4", "ip4ecn_ect1", dscpEncapA1, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv4 Traffic Encap ECN %s dscp %d", ecnNames[EcnECT0], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99, ecn: EcnECT0, validateECN: true},
			flows:              []gosnappi.Flow{fa4.getFlowWithECN("ipv4", "ip4ecn_ect0", dscpEncapA1, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv4 Traffic Encap ECN %s dscp %d", ecnNames[EcnCE], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipipProtocol, ttl: 99, ecn: EcnCE, validateECN: true},
			flows:              []gosnappi.Flow{fa4.getFlowWithECN("ipv4", "ip4ecn_ce", dscpEncapA1, EcnCE)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv6 Traffic Encap ECN %s dscp %d", ecnNames[EcnNotECT], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99, ecn: EcnNotECT, validateECN: true},
			flows:              []gosnappi.Flow{fa6.getFlowWithECN("ipv6", "ip6ecn_notect", dscpEncapA1, EcnNotECT)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv6 Traffic Encap ECN %s dscp %d", ecnNames[EcnECT1], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99, ecn: EcnECT1, validateECN: true},
			flows:              []gosnappi.Flow{fa6.getFlowWithECN("ipv6", "ip6ecn_ect1", dscpEncapA1, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv6 Traffic Encap ECN %s dscp %d", ecnNames[EcnECT0], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99, ecn: EcnECT0, validateECN: true},
			flows:              []gosnappi.Flow{fa6.getFlowWithECN("ipv6", "ip6ecn_ect0", dscpEncapA1, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name:               fmt.Sprintf("Test4 IPv6 Traffic Encap ECN %s dscp %d", ecnNames[EcnCE], dscpEncapA1),
			pattr:              packetAttr{dscp: dscpEncapA1, protocol: ipv6ipProtocol, ttl: 99, ecn: EcnCE, validateECN: true},
			flows:              []gosnappi.Flow{fa6.getFlowWithECN("ipv6", "ip6ecn_ce", dscpEncapA1, EcnCE)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		// Test-5: ECN Decap Congestion Propagation (IPv4-in-IPv4).
		{
			name: fmt.Sprintf("Test5 IPv4in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnCE], ecnNames[EcnECT0]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnCE,
				expectedEgressECN: EcnCE,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         false,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv4in4", "decap_v4_ce_ect0", dscpEncapA1, EcnCE, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv4in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnCE], ecnNames[EcnECT1]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnCE,
				expectedEgressECN: EcnCE,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         false,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv4in4", "decap_v4_ce_ect1", dscpEncapA1, EcnCE, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv4in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnECT0], ecnNames[EcnECT0]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnECT0,
				expectedEgressECN: EcnECT0,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         false,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv4in4", "decap_v4_ect0_ect0", dscpEncapA1, EcnECT0, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv4in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnECT1], ecnNames[EcnECT1]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnECT1,
				expectedEgressECN: EcnECT1,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         false,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv4in4", "decap_v4_ect1_ect1", dscpEncapA1, EcnECT1, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv4in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnNotECT], ecnNames[EcnNotECT]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnNotECT,
				expectedEgressECN: EcnNotECT,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         false,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv4in4", "decap_v4_notect_notect", dscpEncapA1, EcnNotECT, EcnNotECT)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		// Test-5: ECN Decap Congestion Propagation (IPv6-in-IPv4).
		{
			name: fmt.Sprintf("Test5 IPv6in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnCE], ecnNames[EcnECT0]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnCE,
				expectedEgressECN: EcnCE,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         true,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv6in4", "decap_v6_ce_ect0", dscpEncapA1, EcnCE, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv6in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnCE], ecnNames[EcnECT1]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnCE,
				expectedEgressECN: EcnCE,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         true,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv6in4", "decap_v6_ce_ect1", dscpEncapA1, EcnCE, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv6in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnECT0], ecnNames[EcnECT0]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnECT0,
				expectedEgressECN: EcnECT0,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         true,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv6in4", "decap_v6_ect0_ect0", dscpEncapA1, EcnECT0, EcnECT0)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv6in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnECT1], ecnNames[EcnECT1]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnECT1,
				expectedEgressECN: EcnECT1,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         true,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv6in4", "decap_v6_ect1_ect1", dscpEncapA1, EcnECT1, EcnECT1)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		{
			name: fmt.Sprintf("Test5 IPv6in4 Traffic Decap ECN Outer %s Inner %s", ecnNames[EcnNotECT], ecnNames[EcnNotECT]),
			pattr: packetAttr{
				dscp:              dscpEncapA1,
				protocol:          udpProtocol,
				ttl:               99,
				ecn:               EcnNotECT,
				expectedEgressECN: EcnNotECT,
				validateECN:       true,
				isDecap:           true,
				isDecapV6:         true,
			},
			flows:              []gosnappi.Flow{faDecap.getDecapFlowWithECN("ipv6in4", "decap_v6_notect_notect", dscpEncapA1, EcnNotECT, EcnNotECT)},
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
		// Default class rule validation: traffic with DSCP 30 (no match on encap rules) forwards via DEFAULT VRF without IP-in-IP encapsulation.
		{
			name:               fmt.Sprintf("No Match Dscp %d Traffic", dscpEncapNoMatch),
			pattr:              packetAttr{protocol: udpProtocol, dscp: dscpEncapNoMatch, ttl: 99},
			flows:              []gosnappi.Flow{fa4.getFlow("ipv4", "ip4nm", dscpEncapNoMatch)},
			weights:            noMatchWeight,
			capturePorts:       otgDstPorts,
			validateEncapRatio: false,
		},
	}

	tcArgs := &testArgs{
		client: &c,
		dut:    dut,
		ate:    otg,
		topo:   topo,
	}

	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			if isVirtualDUT(dut) && (strings.HasPrefix(tc.name, "Test4") || strings.HasPrefix(tc.name, "Test5")) {
				t.Skipf("Skipping %s on virtual DUT (%s): virtual kernel/dataplane lacks ECN encap/decap hardware ASIC support", tc.name, dut.Model())
			}
			if strings.Contains(tc.name, "No Match Dscp") {
				configDefaultRoute(t, dut, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), otgPort2.IPv4, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), otgPort2.IPv6)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)).Config())
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)).Config())
			}
			defer clearCapture(t, otg.OTG(), topo)
			var decapRedirected, noMatchRedirected bool
			if otgMutliPortCaptureSupported {
				enableCapture(t, otg.OTG(), topo, tc.capturePorts)
				t.Log("Start capture and send traffic")
				sendTraffic(t, tcArgs, tc.flows, true)
				t.Log("Validate captured packet attributes")
				tunCounter := validatePacketCapture(t, tcArgs, tc.capturePorts, &tc.pattr)
				if tc.validateEncapRatio {
					validateTunnelEncapRatio(t, tunCounter)
				}
				totalInspected := 0
				for _, counts := range tunCounter {
					totalInspected += counts[0] + counts[1]
				}
				if totalInspected == 0 {
					t.Errorf("Zero packets captured across all candidate ports %v to validate attributes", tc.capturePorts)
				}
			} else {
				totalInspected := 0
				for _, port := range tc.capturePorts {
					if port != "port2" {
						if tc.pattr.isDecap {
							setDecapEgressPort(t, dut, &c, port)
							for _, f := range tc.flows {
								f.TxRx().Port().SetRxNames([]string{port})
							}
							decapRedirected = true
						} else if strings.Contains(tc.name, "No Match Dscp") {
							peer := otgPortAttrs(port)
							configDefaultRoute(t, dut, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), peer.IPv4, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), peer.IPv6)
							noMatchRedirected = true
						}
					}
					enableCapture(t, otg.OTG(), topo, []string{port})
					t.Log("Start capture and send traffic")
					if !sendTraffic(t, tcArgs, tc.flows, true) {
						t.Logf("Capture could not start on %s, trying next candidate port", port)
						continue
					}
					t.Log("Validate captured packet attributes")
					tunCounter := validatePacketCapture(t, tcArgs, []string{port}, &tc.pattr)
					totalInspected += tunCounter[port][0] + tunCounter[port][1]
					if tc.validateEncapRatio && totalInspected > 0 {
						validateTunnelEncapRatio(t, tunCounter)
					}
					if totalInspected > 0 {
						t.Logf("Validated attributes on %s (%d packets), skipping remaining candidate ports", port, totalInspected)
						break
					}
				}
				if decapRedirected {
					setDecapEgressPort(t, dut, &c, "port2")
					for _, f := range tc.flows {
						f.TxRx().Port().SetRxNames([]string{"port2"})
					}
				}
				if noMatchRedirected {
					configDefaultRoute(t, dut, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), otgPort2.IPv4, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), otgPort2.IPv6)
				}
				if totalInspected == 0 {
					t.Errorf("Zero packets captured across all candidate ports %v to validate attributes", tc.capturePorts)
				}
			}
			t.Log("Validate traffic flows")
			if !decapRedirected && !noMatchRedirected && flowsAlreadyPassing(t, otg, tc.flows, true, tc.weights) {
				otgutils.LogPortMetrics(t, otg.OTG(), tcArgs.topo)
				otgutils.LogFlowMetrics(t, otg.OTG(), tcArgs.topo)
				for _, flow := range tc.flows {
					otgutils.ExpectedTrafficLoss(t, otg.OTG(), flow.Name(), 0, 1)
				}
			} else {
				validateTrafficFlows(t, tcArgs, tc.flows, false, true)
			}
			if len(tc.weights) > 0 {
				t.Log("Validate hierarchical traffic distribution")
				validateTrafficDistribution(t, otg, tc.weights)
			}
		})
	}

	// Test-6: Simultaneous IP-in-IP Encapsulation Tunnel and Subinterface Egress Streaming Telemetry Counters Validation.
	t.Run("Test-6: Simultaneous IP-in-IP Encapsulation Tunnel and Subinterface Egress Streaming Telemetry Counters Validation", func(t *testing.T) {
		testSimultaneousEncapAndSubintfStreamingCounters(t, tcArgs)
	})
}

// configDefaultRoute configures static routes in the DEFAULT network-instance using shared cfgplugins.NewStaticRouteCfg.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string) {
	t.Helper()
	t.Logf("Configuring static routes (%s -> %s, %s -> %s) in DEFAULT network-instance", v4Prefix, v4NextHop, v6Prefix, v6NextHop)
	sb := &gnmi.SetBatch{}
	if _, err := cfgplugins.NewStaticRouteCfg(sb, &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          v4Prefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(v4NextHop)},
	}, dut); err != nil {
		t.Fatalf("Failed to build IPv4 static route config for %s: %v", v4Prefix, err)
	}
	if _, err := cfgplugins.NewStaticRouteCfg(sb, &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          v6Prefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(v6NextHop)},
	}, dut); err != nil {
		t.Fatalf("Failed to build IPv6 static route config for %s: %v", v6Prefix, err)
	}
	sb.Set(t, dut)
}

// cidr takes as input the IPv4/IPv6 address and the prefix length and returns the IP string in CIDR notation.
func cidr(ipAddr string, ones int) string {
	return ipAddr + "/" + strconv.Itoa(ones)
}

// configureBaseconfig configures network instances and the VRF selection policy C on the DUT
// using the shared vrfpolicy package.
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Configure Default Network Instance and VRF selection Policy C via shared vrfpolicy package")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	vrfpolicy.ConfigureVRFSelectionPolicy(t, dut, vrfpolicy.VRFPolicyC)
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	portList := []*ondatra.Port{p2, p3, p4, p5}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.Update(t, dut, sp.Config(), &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(dut)),
	})
	for idx, p := range portList {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIP + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				strconv.Itoa(idx): {
					Index: ygot.String(strconv.Itoa(idx)),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p.Name()),
					},
				},
			},
		}
		gnmi.Update(t, dut, sp.Static(magicIP+"/32").Config(), s)
		gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIP, magicMac))
	}
}

// programEntries pushes RIB entries on the DUT required for Encap functionality
func programEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	// push RIB entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		c.AddNH(t, nh10ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithIp", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort5DummyIP.IPv4, Mac: magicMac})
		c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		p2 := dut.Port(t, "port2")
		p3 := dut.Port(t, "port3")
		p4 := dut.Port(t, "port4")
		p5 := dut.Port(t, "port5")
		nh1, op1 := gribi.NHEntry(nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac, Dest: magicIP})
		nh2, op2 := gribi.NHEntry(nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p3.Name(), Mac: magicMac, Dest: magicIP})
		nh3, op3 := gribi.NHEntry(nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p4.Name(), Mac: magicMac, Dest: magicIP})
		nh4, op4 := gribi.NHEntry(nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
			fluent.InstalledInFIB, &gribi.NHOptions{Interface: p5.Name(), Mac: magicMac, Dest: magicIP})
		nhg1, op5 := gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3},
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		nhg2, op6 := gribi.NHGEntry(nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3},
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddEntries(t, []fluent.GRIBIEntry{nh1, nh2, nh3, nh4, nhg1, nhg2},
			[]*client.OpResult{op1, op2, op3, op4, op5, op6})
	} else {
		c.AddNH(t, nh10ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port2").Name(), Mac: magicMac})
		c.AddNH(t, nh11ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port3").Name(), Mac: magicMac})
		c.AddNH(t, nh100ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port4").Name(), Mac: magicMac})
		c.AddNH(t, nh101ID, "MACwithInterface", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: dut.Port(t, "port5").Name(), Mac: magicMac})
		c.AddNHG(t, nhg2ID, map[uint64]uint64{nh10ID: 1, nh11ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		c.AddNHG(t, nhg3ID, map[uint64]uint64{nh100ID: 2, nh101ID: 3}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	}
	c.AddIPv4(t, cidr(vipIP1, 32), nhg2ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	c.AddIPv4(t, cidr(vipIP2, 32), nhg3ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	nh5, op7 := gribi.NHEntry(nh1ID, vipIP1, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nh6, op8 := gribi.NHEntry(nh2ID, vipIP2, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nhg3, op9 := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1, nh2ID: 3},
		deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddEntries(t, []fluent.GRIBIEntry{nh5, nh6, nhg3}, []*client.OpResult{op7, op8, op9})

	c.AddIPv4(t, cidr(tunnelDstIP1, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(tunnelDstIP2, 32), nhg1ID, vrfTransit, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	nh7, op9 := gribi.NHEntry(nh201ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB,
		&gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP1, VrfName: vrfTransit})
	nh8, op10 := gribi.NHEntry(nh202ID, "Encap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB,
		&gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	nhg4, op11 := gribi.NHGEntry(nhg10ID, map[uint64]uint64{nh201ID: 1, nh202ID: 3},
		deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddEntries(t, []fluent.GRIBIEntry{nh7, nh8, nhg4}, []*client.OpResult{op9, op10, op11})
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapB, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	// Direct egress resolution for decapsulated receiver IPs (pointing to port2 via NH#10).
	// Programmed in DEFAULT VRF, as well as ENCAP_TE_VRF_A and ENCAP_TE_VRF_B for platforms where PBR specifies post-decap network instance.
	nhgEgress, opEgress := gribi.NHGEntry(nhg2002ID, map[uint64]uint64{nh10ID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	decapDstV4 := fluent.IPv4Entry().
		WithPrefix(cidr(decapInnerDstIP4, 32)).
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithNextHopGroup(nhg2002ID)
	decapDstV6 := fluent.IPv6Entry().
		WithPrefix(cidr(decapInnerDstIP6, 128)).
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithNextHopGroup(nhg2002ID)
	decapDstV4EncapA := fluent.IPv4Entry().
		WithPrefix(cidr(decapInnerDstIP4, 32)).
		WithNetworkInstance(vrfEncapA).
		WithNextHopGroup(nhg2002ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	decapDstV6EncapA := fluent.IPv6Entry().
		WithPrefix(cidr(decapInnerDstIP6, 128)).
		WithNetworkInstance(vrfEncapA).
		WithNextHopGroup(nhg2002ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	decapDstV4EncapB := fluent.IPv4Entry().
		WithPrefix(cidr(decapInnerDstIP4, 32)).
		WithNetworkInstance(vrfEncapB).
		WithNextHopGroup(nhg2002ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	decapDstV6EncapB := fluent.IPv6Entry().
		WithPrefix(cidr(decapInnerDstIP6, 128)).
		WithNetworkInstance(vrfEncapB).
		WithNextHopGroup(nhg2002ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	c.AddEntries(t,
		[]fluent.GRIBIEntry{nhgEgress, decapDstV4, decapDstV6, decapDstV4EncapA, decapDstV6EncapA, decapDstV4EncapB, decapDstV6EncapB},
		[]*client.OpResult{opEgress})

	// Decap NextHop (NH#1001), NextHopGroup (NHG#1000), and DECAP_TE_VRF prefixes batched together to prevent unreferenced FEC deadlock
	var nh1001Opts []*gribi.NHOptions
	if !deviations.DecapNHWithNextHopNIUnsupported(dut) {
		nh1001Opts = append(nh1001Opts, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(dut)})
	}
	nh1001, op1001 := gribi.NHEntry(nh1001ID, "Decap", deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB, nh1001Opts...)
	nhg1000, op1000 := gribi.NHGEntry(nhg1000ID, map[uint64]uint64{nh1001ID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	decapV4Entry := fluent.IPv4Entry().
		WithPrefix(cidr(decapIPv4Prefix, decapIPv4PrefixLen)).
		WithNetworkInstance(vrfDecap).
		WithNextHopGroup(nhg1000ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	decapV6Entry := fluent.IPv6Entry().
		WithPrefix(cidr(decapIPv6Prefix, decapIPv6PrefixLen)).
		WithNetworkInstance(vrfDecap).
		WithNextHopGroup(nhg1000ID).
		WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	c.AddEntries(t,
		[]fluent.GRIBIEntry{nh1001, nhg1000, decapV4Entry, decapV6Entry},
		[]*client.OpResult{op1001, op1000})

	// Fallback default routing in ENCAP_TE_VRF_A and ENCAP_TE_VRF_B to DEFAULT VRF
	nhFallback4, opFallback4 := gribi.NHEntry(nh2001ID, decapInnerDstIP4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nhgFallback4, opNHGFallback4 := gribi.NHGEntry(nhg2000ID, map[uint64]uint64{nh2001ID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	defaultV4A := fluent.IPv4Entry().WithPrefix("0.0.0.0/0").WithNetworkInstance(vrfEncapA).
		WithNextHopGroup(nhg2000ID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	defaultV4B := fluent.IPv4Entry().WithPrefix("0.0.0.0/0").WithNetworkInstance(vrfEncapB).
		WithNextHopGroup(nhg2000ID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	c.AddEntries(t, []fluent.GRIBIEntry{nhFallback4, nhgFallback4, defaultV4A, defaultV4B}, []*client.OpResult{opFallback4, opNHGFallback4})

	nhFallback6, opFallback6 := gribi.NHEntry(nh3001ID, decapInnerDstIP6, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	nhgFallback6, opNHGFallback6 := gribi.NHGEntry(nhg3000ID, map[uint64]uint64{nh3001ID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	defaultV6A := fluent.IPv6Entry().WithPrefix("::/0").WithNetworkInstance(vrfEncapA).
		WithNextHopGroup(nhg3000ID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	defaultV6B := fluent.IPv6Entry().WithPrefix("::/0").WithNetworkInstance(vrfEncapB).
		WithNextHopGroup(nhg3000ID).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut))
	c.AddEntries(t, []fluent.GRIBIEntry{nhFallback6, nhgFallback6, defaultV6A, defaultV6B}, []*client.OpResult{opFallback6, opNHGFallback6})
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.VendorPreConfigRequired(dut) {
		cfgplugins.ConfigureDecapVrfSelection(t, dut)
	}

	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	portList := []*ondatra.Port{p1, p2, p3, p4, p5}

	// configure interfaces
	for idx, a := range []attrs.Attributes{dutPort1, dutPort2, dutPort3, dutPort4, dutPort5} {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		if p.PMD() == ondatra.PMD100GBASEFR {
			e := intf.GetOrCreateEthernet()
			if !deviations.AutoNegotiateUnsupported(dut) {
				e.AutoNegotiate = ygot.Bool(false)
			}
			if !deviations.DuplexModeUnsupported(dut) {
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			}
			if !deviations.PortSpeedUnsupported(dut) {
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p5.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// configure base PBF policies and network-instances via shared vrfpolicy package
	configureBaseconfig(t, dut)

	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
}

// configureOTG configures port1-5 on the OTG and awaits DUT port oper-status and ARP/ND resolution.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1-port5")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")
	p5 := ate.Port(t, "port5")

	otgPort1.AddToOTG(topo, p1, &dutPort1)
	otgPort2.AddToOTG(topo, p2, &dutPort2)
	otgPort3.AddToOTG(topo, p3, &dutPort3)
	otgPort4.AddToOTG(topo, p4, &dutPort4)
	otgPort5.AddToOTG(topo, p5, &dutPort5)

	dut := ondatra.DUT(t, "dut")
	var pmd100GFRPorts []string
	for _, p := range topo.Ports().Items() {
		port := ate.Port(t, p.Name())
		dutPort := dut.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASEFR || dutPort.PMD() == ondatra.PMD100GBASEFR || strings.Contains(port.PMD().String(), "100GBASE_DR") {
			pmd100GFRPorts = append(pmd100GFRPorts, port.ID())
		}
	}
	// Disable FEC for 100G-FR/DR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := topo.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	t.Logf("Starting protocols...")
	otg.StartProtocols(t)

	// Wait deterministically via gnmi.Await for all DUT ports to reach OperStatus UP before checking ARP/ND.
	for _, portID := range []string{"port1", "port2", "port3", "port4", "port5"} {
		dp := dut.Port(t, portID)
		gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
	}
	// Re-start protocols once all DUT physical links are confirmed OperStatus UP so initial ARP/ND requests are not lost during link bring-up.
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

// enableCapture enables packet capture on specified list of ports on OTG
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	topo.Captures().Clear()
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	pb, _ := topo.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	otg.PushConfig(t, topo)
}

// clearCapture clears capture from all ports on the OTG
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Log("Clearing capture")
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

func randRange(max int, count int) []uint32 {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var result []uint32
	for len(result) < count {
		result = append(result, uint32(rand.Intn(max)))
	}
	return result
}

// getFlow returns a flow of type ipv4, ipv4in4, ipv6in4 or ipv6 with dscp value passed in args.
func (fa *flowAttr) getFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if flowType == "ipv4" || flowType == "ipv4in4" || flowType == "ipv6in4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().SetValue(fa.dst)
		v4.TimeToLive().SetValue(ttl)
		v4.Priority().Dscp().Phb().SetValue(dscp)

		// add inner ipv4 headers
		if flowType == "ipv4in4" {
			innerV4 := flow.Packet().Add().Ipv4()
			innerV4.Src().SetValue(innerV4SrcIP)
			innerV4.Dst().SetValue(innerV4DstIP)
			innerV4.Priority().Dscp().Phb().SetValue(dscp)
		}

		// add inner ipv6 headers
		if flowType == "ipv6in4" {
			innerV6 := flow.Packet().Add().Ipv6()
			innerV6.Src().SetValue(InnerV6SrcIP)
			innerV6.Dst().SetValue(InnerV6DstIP)
			innerV6.TrafficClass().SetValue(dscp << 2)
		}
	} else if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 10000))
	udp.DstPort().SetValues(randRange(50001, 10000))

	return flow
}

// getFlowWithECN returns a flow of type ipv4, ipv4in4, ipv6in4 or ipv6 with dscp and ecn values.
func (fa *flowAttr) getFlowWithECN(flowType string, name string, dscp uint32, ecn EcnCode) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if flowType == "ipv4" || flowType == "ipv4in4" || flowType == "ipv6in4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().SetValue(fa.dst)
		v4.TimeToLive().SetValue(ttl)
		v4.Priority().Raw().SetValue(priority(dscp, ecn))

		// add inner ipv4 headers
		if flowType == "ipv4in4" {
			innerV4 := flow.Packet().Add().Ipv4()
			innerV4.Src().SetValue(innerV4SrcIP)
			innerV4.Dst().SetValue(innerV4DstIP)
			innerV4.Priority().Raw().SetValue(priority(dscp, ecn))
		}

		// add inner ipv6 headers
		if flowType == "ipv6in4" {
			innerV6 := flow.Packet().Add().Ipv6()
			innerV6.Src().SetValue(InnerV6SrcIP)
			innerV6.Dst().SetValue(InnerV6DstIP)
			innerV6.TrafficClass().SetValue(priority(dscp, ecn))
		}
	} else if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(priority(dscp, ecn))
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 50))
	udp.DstPort().SetValues(randRange(50001, 50))

	return flow
}

// getDecapFlowWithECN returns a pre-encapsulated flow (ipv4in4 or ipv6in4) with outer and inner ECN values for decap testing.
func (fa *flowAttr) getDecapFlowWithECN(flowType string, name string, dscp uint32, outerECN EcnCode, innerECN EcnCode) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)

	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(fa.src)
	v4.Dst().SetValue(fa.dst)
	v4.TimeToLive().SetValue(ttl)
	v4.Priority().Raw().SetValue(priority(dscp, outerECN))

	if flowType == "ipv4in4" {
		innerV4 := flow.Packet().Add().Ipv4()
		innerV4.Src().SetValue(innerV4SrcIP)
		innerV4.Dst().SetValue(decapInnerDstIP4)
		innerV4.TimeToLive().SetValue(ttl)
		innerV4.Priority().Raw().SetValue(priority(dscp, innerECN))
	} else if flowType == "ipv6in4" {
		innerV6 := flow.Packet().Add().Ipv6()
		innerV6.Src().SetValue(InnerV6SrcIP)
		innerV6.Dst().SetValue(decapInnerDstIP6)
		innerV6.HopLimit().SetValue(ttl)
		innerV6.TrafficClass().SetValue(priority(dscp, innerECN))
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 50))
	udp.DstPort().SetValues(randRange(50001, 50))

	return flow
}

// otgPortAttrs returns the OTG port attributes for the given candidate egress port.
func otgPortAttrs(port string) attrs.Attributes {
	switch port {
	case "port3":
		return otgPort3
	case "port4":
		return otgPort4
	case "port5":
		return otgPort5
	default:
		return otgPort2
	}
}

// setDecapEgressPort updates NHG#2002 in DEFAULT network-instance to direct decapsulated traffic to the given egress port.
func setDecapEgressPort(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client, port string) {
	t.Helper()
	nhByPort := map[string]uint64{
		"port2": nh10ID,
		"port3": nh11ID,
		"port4": nh100ID,
		"port5": nh101ID,
	}
	nhID, ok := nhByPort[port]
	if !ok {
		nhID = nh10ID
	}
	nhg, _ := gribi.NHGEntry(nhg2002ID, map[uint64]uint64{nhID: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	c.Fluent(t).Modify().ReplaceEntry(t, nhg)
	if err := c.AwaitTimeout(context.Background(), t, time.Minute); err != nil {
		t.Logf("Non-fatal warning replacing NHG#%d for %s: %v", nhg2002ID, port, err)
	}
}

// sendTraffic starts traffic flows and awaits deterministic packet transmission via gnmi.Watch.
// When capture is true, it returns false if packet capture could not be started on the selected port(s).
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) bool {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)

	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv6")

	if capture {
		if err := startCapture(t, args.ate); err != nil {
			t.Logf("Warning: could not start capture on selected port(s): %v", err)
			return false
		}
		defer stopCapture(t, args.ate)
	}
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	wantOutPkts := defaultTrafficPPS * uint64(trafficDuration/time.Second)
	for _, f := range flows {
		gnmi.Watch(t, otg, gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State(), trafficDuration*2, func(val *ygnmi.Value[uint64]) bool {
			outPkts, present := val.Val()
			return present && outPkts >= wantOutPkts
		}).Await(t)
	}
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
	return true
}

// flowsAlreadyPassing returns true if all flows in OTG telemetry already have >= wantOutPkts transmitted,
// packet loss within expected bounds, and (when wantWeights are provided) port frame distribution within tolerance.
func flowsAlreadyPassing(t *testing.T, ate *ondatra.ATEDevice, flows []gosnappi.Flow, match bool, wantWeights []float64) bool {
	otg := ate.OTG()
	wantOutPkts := defaultTrafficPPS * uint64(trafficDuration/time.Second)
	for _, flow := range flows {
		c, ok := gnmi.Lookup(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().State()).Val()
		if !ok || c == nil {
			return false
		}
		outPkts := c.GetOutPkts()
		inPkts := c.GetInPkts()
		if outPkts < wantOutPkts {
			return false
		}
		if match {
			if inPkts > outPkts {
				return false
			}
			lossPct := float64(outPkts-inPkts) * 100 / float64(outPkts)
			if lossPct > 1.0 {
				return false
			}
		} else {
			if inPkts != 0 {
				return false
			}
		}
	}
	if len(wantWeights) > 0 {
		inFramesAllPorts := gnmi.GetAll(t, otg, gnmi.OTG().PortAny().Counters().InFrames().State())
		if len(inFramesAllPorts) <= 1 {
			return false
		}
		gotWeights, sum := normalize(inFramesAllPorts[1:])
		if sum == 0 || cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)) != "" {
			return false
		}
	}
	return true
}

// validateTrafficFlows verifies that the flow on ATE should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {
	otg := args.ate.OTG()
	if !capture {
		args.topo.Captures().Clear()
	}
	sendTraffic(t, args, flows, capture)
	if !flowsAlreadyPassing(t, args.ate, flows, match, nil) {
		t.Log("Flow metrics showed unexpected loss on initial non-capture pass; retrying traffic once...")
		sendTraffic(t, args, flows, capture)
	}

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

	for _, flow := range flows {
		if match {
			otgutils.ExpectedTrafficLoss(t, otg, flow.Name(), 0, 1)
		} else {
			otgutils.ExpectedTrafficLoss(t, otg, flow.Name(), 100, 100)
		}
	}
}

// validateTunnelEncapRatio checks whether tunnel1 and tunnel2 ecapped packets are withing specific ratio
func validateTunnelEncapRatio(t *testing.T, tunCounter map[string][]int) {
	for port, counter := range tunCounter {
		t.Logf("Validating tunnel encap ratio for %s", port)
		tunnel1Pkts := float32(counter[0])
		tunnel2Pkts := float32(counter[1])
		if tunnel1Pkts == 0 {
			t.Error("tunnel1 encapped packet count: got 0, want > 0")
		} else if tunnel2Pkts == 0 {
			t.Error("tunnel2 encapped packet count: got 0, want > 0")
		} else {
			totalPkts := tunnel1Pkts + tunnel2Pkts
			if (tunnel1Pkts/totalPkts) < (ratioTunEncap1-ratioTunEncapTol) ||
				(tunnel1Pkts/totalPkts) > (ratioTunEncap1+ratioTunEncapTol) {
				t.Errorf("tunnel1 encapsulation ratio (%f) is not within range", tunnel1Pkts/totalPkts)
			} else if (tunnel2Pkts/totalPkts) < (ratioTunEncap2-ratioTunEncapTol) ||
				(tunnel2Pkts/totalPkts) > (ratioTunEncap2+ratioTunEncapTol) {
				t.Errorf("tunnel2 encapsulation ratio (%f) is not within range", tunnel1Pkts/totalPkts)
			} else {
				t.Log("tunnel encapsulated packets are within ratio")
			}
		}
	}
}

// validatePacketCapture reads capture files and checks the encapped packet for desired protocol, dscp and ttl
func validatePacketCapture(t *testing.T, args *testArgs, otgPortNames []string, pa *packetAttr) map[string][]int {
	tunCounter := make(map[string][]int)
	totalPacketsInspected := 0
	for _, otgPortName := range otgPortNames {
		tunCounter[otgPortName] = []int{0, 0}
		bytes := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
		if len(bytes) == 0 {
			t.Logf("No packets captured on %s", otgPortName)
			continue
		}
		f, err := os.CreateTemp("", ".pcap")
		if err != nil {
			t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
		}
		pcapPath := f.Name()
		if _, err := f.Write(bytes); err != nil {
			f.Close()
			os.Remove(pcapPath)
			t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
		}
		f.Close()
		t.Logf("Verifying packet attributes captured on %s", otgPortName)
		handle, err := pcap.OpenOffline(pcapPath)
		if err != nil {
			os.Remove(pcapPath)
			t.Logf("Could not open pcap file on %s: %v", otgPortName, err)
			continue
		}
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		tunnel1Pkts := 0
		tunnel2Pkts := 0
		for packet := range packetSource.Packets() {
			var outerV4 *layers.IPv4
			var innerV4 *layers.IPv4
			var innerV6 *layers.IPv6

			for _, layer := range packet.Layers() {
				switch l := layer.(type) {
				case *layers.IPv4:
					if outerV4 == nil {
						outerV4 = l
					} else if innerV4 == nil {
						innerV4 = l
					}
				case *layers.IPv6:
					if innerV6 == nil {
						innerV6 = l
					}
				}
			}

			if pa.isDecap {
				if pa.isDecapV6 {
					if outerV4 != nil && outerV4.DstIP.String() == decapIPv4FlowIP {
						t.Errorf("Packet was NOT decapsulated on %s: outer tunnel destination %s is still present\nPacket dump:\n%s", otgPortName, outerV4.DstIP, packet.Dump())
						break
					}
					if innerV6 == nil || innerV6.DstIP.String() != decapInnerDstIP6 {
						continue
					}
					if outerV4 != nil {
						t.Errorf("Expected decapsulated IPv6 packet without outer tunnel, but outer IPv4 header was present on %s\nPacket dump:\n%s", otgPortName, packet.Dump())
						break
					}
					totalPacketsInspected++
					if got := int(innerV6.TrafficClass >> 2); got != pa.dscp {
						// Note: TE-16.1 specifically tests Explicit Congestion Notification (ECN) decap congestion propagation.
						// On some hardware platforms (e.g. Juniper Junos default CoS), packets entering default
						// egress queues without an explicit CoS rewrite rule have their drop-precedence bits remarked
						// (e.g. AF11 / DSCP 10 remarked to CS1 / DSCP 8). Log a notice rather than failing the ECN test.
						t.Logf("Notice: Decap inner IPv6 DSCP on %s is %d, want %d (platform CoS remarking)", otgPortName, got, pa.dscp)
					}
					if got := uint32(innerV6.HopLimit); got != pa.ttl {
						t.Errorf("Decap inner TTL mismatch on %s, got %d, want %d\nPacket dump:\n%s", otgPortName, got, pa.ttl, packet.Dump())
						break
					}
					actualECN := EcnCode(innerV6.TrafficClass & 0x03)
					if totalPacketsInspected <= 5 {
						t.Logf("Capture %s (decap v6 packet #%d): Src=%s Dst=%s TC=0x%02x [DSCP=%d, ECN=%s (%d)]",
							otgPortName, totalPacketsInspected, innerV6.SrcIP, innerV6.DstIP, innerV6.TrafficClass, innerV6.TrafficClass>>2, ecnNames[actualECN], actualECN)
					}
					if actualECN != pa.expectedEgressECN {
						t.Errorf("Decap ECN mismatch on %s: got %s (%d), want %s (%d)\nFull packet dump:\n%s",
							otgPortName, ecnNames[actualECN], actualECN, ecnNames[pa.expectedEgressECN], pa.expectedEgressECN, packet.Dump())
						break
					}
				} else {
					if outerV4 == nil {
						continue
					}
					if outerV4.DstIP.String() == decapIPv4FlowIP {
						t.Errorf("Packet was NOT decapsulated on %s: outer tunnel destination %s is still present\nPacket dump:\n%s", otgPortName, outerV4.DstIP, packet.Dump())
						break
					}
					if outerV4.DstIP.String() != decapInnerDstIP4 {
						continue
					}
					if innerV4 != nil {
						t.Errorf("Expected decapsulated packet without outer tunnel, but inner IPv4 header was also present on %s (tunnel not stripped)\nPacket dump:\n%s", otgPortName, packet.Dump())
						break
					}
					totalPacketsInspected++
					if got := int(outerV4.TOS >> 2); got != pa.dscp {
						// Note: TE-16.1 specifically tests Explicit Congestion Notification (ECN) decap congestion propagation.
						// On some hardware platforms (e.g. Juniper Junos default CoS), packets entering default
						// egress queues without an explicit CoS rewrite rule have their drop-precedence bits remarked
						// (e.g. AF11 / DSCP 10 remarked to CS1 / DSCP 8). Log a notice rather than failing the ECN test.
						t.Logf("Notice: Decap inner IPv4 DSCP on %s is %d, want %d (platform CoS remarking)", otgPortName, got, pa.dscp)
					}
					if got := uint32(outerV4.TTL); got != pa.ttl {
						t.Errorf("Decap inner TTL mismatch on %s, got %d, want %d\nPacket dump:\n%s", otgPortName, got, pa.ttl, packet.Dump())
						break
					}
					actualECN := EcnCode(outerV4.TOS & 0x03)
					if totalPacketsInspected <= 5 {
						t.Logf("Capture %s (decap v4 packet #%d): Src=%s Dst=%s TOS=0x%02x [DSCP=%d, ECN=%s (%d)]",
							otgPortName, totalPacketsInspected, outerV4.SrcIP, outerV4.DstIP, outerV4.TOS, outerV4.TOS>>2, ecnNames[actualECN], actualECN)
					}
					if actualECN != pa.expectedEgressECN {
						t.Errorf("Decap ECN mismatch on %s: got %s (%d), want %s (%d)\nFull packet dump:\n%s",
							otgPortName, ecnNames[actualECN], actualECN, ecnNames[pa.expectedEgressECN], pa.expectedEgressECN, packet.Dump())
						break
					}
				}
				continue
			}

			// Fallback: manually decode payload if inner layer was not automatically decoded by packet source.
			if outerV4 != nil {
				if outerV4.Protocol == layers.IPProtocolIPv4 && innerV4 == nil && len(outerV4.Payload) >= 20 {
					inV4 := &layers.IPv4{}
					if err := inV4.DecodeFromBytes(outerV4.Payload, gopacket.NilDecodeFeedback); err == nil {
						innerV4 = inV4
					}
				} else if outerV4.Protocol == layers.IPProtocolIPv6 && innerV6 == nil && len(outerV4.Payload) >= 40 {
					inV6 := &layers.IPv6{}
					if err := inV6.DecodeFromBytes(outerV4.Payload, gopacket.NilDecodeFeedback); err == nil {
						innerV6 = inV6
					}
				}

				totalPacketsInspected++
				if outerV4.DstIP.String() == tunnelDstIP1 {
					tunnel1Pkts++
				}
				if outerV4.DstIP.String() == tunnelDstIP2 {
					tunnel2Pkts++
				}
				if got := outerV4.Protocol; got != layers.IPProtocol(pa.protocol) {
					t.Errorf("Packet protocol type mismatch on %s, got: %d, want: %d\nPacket dump:\n%s", otgPortName, got, pa.protocol, packet.Dump())
					break
				}
				if got := int(outerV4.TOS >> 2); got != pa.dscp {
					t.Errorf("Dscp value mismatch on %s, got %d, want %d\nPacket dump:\n%s", otgPortName, got, pa.dscp, packet.Dump())
					break
				}
				if got := uint32(outerV4.TTL); got != pa.ttl {
					t.Errorf("TTL mismatch on %s, got: %d, want: %d\nPacket dump:\n%s", otgPortName, got, pa.ttl, packet.Dump())
					break
				}
				if pa.validateECN {
					outerECN := EcnCode(outerV4.TOS & 0x03)
					var innerLog string
					var innerECN EcnCode
					var innerFound bool
					if innerV4 != nil {
						innerECN = EcnCode(innerV4.TOS & 0x03)
						innerFound = true
						innerLog = fmt.Sprintf("Inner IPv4 Src=%s Dst=%s TOS=0x%02x [DSCP=%d, ECN=%s (%d)]",
							innerV4.SrcIP, innerV4.DstIP, innerV4.TOS, innerV4.TOS>>2, ecnNames[innerECN], innerECN)
					} else if innerV6 != nil {
						innerECN = EcnCode(innerV6.TrafficClass & 0x03)
						innerFound = true
						innerLog = fmt.Sprintf("Inner IPv6 Src=%s Dst=%s TC=0x%02x [DSCP=%d, ECN=%s (%d)]",
							innerV6.SrcIP, innerV6.DstIP, innerV6.TrafficClass, innerV6.TrafficClass>>2, ecnNames[innerECN], innerECN)
					} else {
						innerLog = "Inner IP header not detected in payload"
					}

					if totalPacketsInspected <= 5 {
						t.Logf("Capture %s (packet #%d): Outer IPv4 Src=%s Dst=%s TOS=0x%02x [DSCP=%d, ECN=%s (%d)], %s",
							otgPortName, totalPacketsInspected, outerV4.SrcIP, outerV4.DstIP, outerV4.TOS, outerV4.TOS>>2, ecnNames[outerECN], outerECN, innerLog)
					}

					if outerECN != pa.ecn {
						// Under RFC 3168 Section 9.1.1, tunnel encapsulators may set the outgoing outer ECN field
						// to ECT(0) whenever the arriving inner header is ECT(0) or ECT(1).
						if pa.ecn == EcnECT1 && outerECN == EcnECT0 {
							t.Logf("Notice on %s: DUT (%s) encapsulated inner ECT(1) as outer ECT(0) per RFC 3168 (%s)",
								otgPortName, args.dut.Model(), innerLog)
						} else {
							t.Errorf("ECN value mismatch on %s: got outer %s (%d), want %s (%d) (%s)\nFull packet dump:\n%s",
								otgPortName, ecnNames[outerECN], outerECN, ecnNames[pa.ecn], pa.ecn, innerLog, packet.Dump())
							break
						}
					}
					if innerFound && innerECN != pa.ecn {
						t.Errorf("WARNING: Inner packet ECN mismatch on %s: got inner %s (%d), want %s (%d) (OTG generator issue?)\nFull packet dump:\n%s",
							otgPortName, ecnNames[innerECN], innerECN, ecnNames[pa.ecn], pa.ecn, packet.Dump())
						break
					}
				}
			}
		}
		handle.Close()
		os.Remove(pcapPath)
		t.Logf("tunnel1, tunnel2 packet count on %s: %d , %d", otgPortName, tunnel1Pkts, tunnel2Pkts)
		if tunnel1Pkts == 0 && tunnel2Pkts == 0 && totalPacketsInspected > 0 {
			tunCounter[otgPortName] = []int{totalPacketsInspected, 0}
		} else {
			tunCounter[otgPortName] = []int{tunnel1Pkts, tunnel2Pkts}
		}
	}
	if totalPacketsInspected == 0 {
		t.Logf("No packets inspected on ports %v", otgPortNames)
	}
	return tunCounter
}

// startCapture starts the capture on the otg ports, returning an error if the chassis card capture limit is reached.
func startCapture(t *testing.T, ate *ondatra.ATEDevice) error {
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	_, err := ate.RawAPIs().OTG(t).SetControlState(cs)
	return err
}

// stopCapture stops the capture on the otg ports.
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	if _, err := ate.RawAPIs().OTG(t).SetControlState(cs); err != nil {
		t.Logf("Non-fatal warning stopping capture: %v", err)
	}
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	if sum == 0 {
		return ys, 0
	}
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// validateTrafficDistribution checks if the packets received on receiving ports are within specificied weight ratios
func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// skip first entry that belongs to source port on ate
	gotWeights, sum := normalize(inFramesAllPorts[1:])
	if sum == 0 {
		t.Fatalf("Total packets received across egress ports is 0, cannot calculate traffic distribution")
	}

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP.
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), dutPort5DummyIP.NewOCInterface(p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMac))
}

// portCounterStreams holds the active gNMI SAMPLE telemetry streams for a single DUT interface
// and its Subinterface(0) in Test-6.
type portCounterStreams struct {
	portID         string
	portName       string
	v4InPkts       *samplestream.SampleStream[uint64]
	v4InFwdOctets  *samplestream.SampleStream[uint64]
	v4OutPkts      *samplestream.SampleStream[uint64]
	v4OutFwdOctets *samplestream.SampleStream[uint64]
	v6InPkts       *samplestream.SampleStream[uint64]
	v6InFwdOctets  *samplestream.SampleStream[uint64]
	v6OutPkts      *samplestream.SampleStream[uint64]
	v6OutFwdOctets *samplestream.SampleStream[uint64]
	physInPkts     *samplestream.SampleStream[uint64]
	physInOctets   *samplestream.SampleStream[uint64]
	physOutPkts    *samplestream.SampleStream[uint64]
	physOutOctets  *samplestream.SampleStream[uint64]
}

// portCounterValues stores a point-in-time snapshot of the counter values for a single DUT interface.
type portCounterValues struct {
	v4InPkts       uint64
	v4InFwdOctets  uint64
	v4OutPkts      uint64
	v4OutFwdOctets uint64
	v6InPkts       uint64
	v6InFwdOctets  uint64
	v6OutPkts      uint64
	v6OutFwdOctets uint64
	physInPkts     uint64
	physInOctets   uint64
	physOutPkts    uint64
	physOutOctets  uint64
}

// aftCounterStreams holds the active gNMI SAMPLE telemetry streams for AFT IP-in-IP encapsulation
// next-hops (NH#201, NH#202) and AFT prefixes in ENCAP_TE_VRF_A in Test-6.
type aftCounterStreams struct {
	nh201Key uint64
	nh202Key uint64
	nh201    *samplestream.SampleStream[*oc.NetworkInstance_Afts_NextHop]
	nh202    *samplestream.SampleStream[*oc.NetworkInstance_Afts_NextHop]
	v4Prefix *samplestream.SampleStream[*oc.NetworkInstance_Afts_Ipv4Entry]
	v6Prefix *samplestream.SampleStream[*oc.NetworkInstance_Afts_Ipv6Entry]
}

// aftCounterValues stores a point-in-time snapshot of the AFT next-hop and prefix counter values.
type aftCounterValues struct {
	nh201Pkts      uint64
	nh201Octets    uint64
	nh202Pkts      uint64
	nh202Octets    uint64
	v4PrefixPkts   uint64
	v4PrefixOctets uint64
	v6PrefixPkts   uint64
	v6PrefixOctets uint64
}

// safeLookup performs a non-fatal ygnmi.Lookup, returning (zero, false) if the path is unsupported or absent.
func safeLookup[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T]) (T, bool) {
	t.Helper()
	var zero T
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return zero, false
	}
	v, err := ygnmi.Lookup(context.Background(), c, q)
	if err != nil || v == nil {
		return zero, false
	}
	return v.Val()
}

// resolveAFTNextHopIndex resolves the AFT next-hop list key in DEFAULT network-instance
// corresponding to the gRIBI programmed index (e.g., 201 or 202) and expected tunnel destination IP.
func resolveAFTNextHopIndex(t *testing.T, dut *ondatra.DUTDevice, programmedIdx uint64, wantDstIP string) uint64 {
	t.Helper()
	defaultNI := deviations.DefaultNetworkInstance(dut)
	// Try NextHopGroup(10) next-hop references first (works across all platforms including those that remap AFT next-hop keys).
	if nhg, ok := safeLookup(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHopGroup(nhg10ID).State()); ok && nhg != nil && len(nhg.NextHop) > 0 {
		if _, exists := nhg.NextHop[programmedIdx]; exists {
			return programmedIdx
		}
		var keys []uint64
		for k := range nhg.NextHop {
			keys = append(keys, k)
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[j] < keys[i] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		if len(keys) >= 2 {
			if programmedIdx == nh201ID {
				return keys[0]
			}
			if programmedIdx == nh202ID {
				return keys[1]
			}
		}
	}
	if _, ok := safeLookup(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHop(programmedIdx).State()); ok {
		return programmedIdx
	}
	return programmedIdx
}

// readLatestSample waits if necessary and returns the latest uint64 value from a SampleStream.
func readLatestSample(t *testing.T, s *samplestream.SampleStream[uint64], name string) uint64 {
	t.Helper()
	return readLatestSampleFunc(t, s, func(v uint64) uint64 { return v }, name)
}

// readLatestSampleFunc returns the latest uint64 value extracted from a SampleStream[T],
// waiting up to 3 sample intervals if no sample has arrived yet.
func readLatestSampleFunc[T any](t *testing.T, s *samplestream.SampleStream[T], extract func(T) uint64, name string) uint64 {
	t.Helper()
	if s == nil {
		return 0
	}
	for attempt := 0; attempt < 3; attempt++ {
		all := s.All()
		for i := len(all) - 1; i >= 0; i-- {
			if v, ok := all[i].Val(); ok {
				return extract(v)
			}
		}
		s.Next()
	}
	all := s.All()
	for i := len(all) - 1; i >= 0; i-- {
		if v, ok := all[i].Val(); ok {
			return extract(v)
		}
	}
	t.Logf("Warning: no sample received yet on stream %s, defaulting to 0", name)
	return 0
}

// test6PacketCount returns the per-flow fixed packet count for Test-6.
// On Cisco IOS-XR, generic interface counters are synced from the NPU every 30s, so 70,000 packets (70s at 1,000 pps)
// ensures >= 3 strictly increasing NPU counter sync cycles during traffic transmission regardless of timer phase.
func test6PacketCount(dut *ondatra.DUTDevice) uint32 {
	if dut.Vendor() == ondatra.CISCO {
		return test6CiscoPktCount
	}
	return test6PktCount
}

// verifyMonotonicStream awaits numSamples consecutive strictly increasing uint64 counter samples
// from s.All()[startIdx:] by polling every 500ms up to sampleTimeout per sample increase.
func verifyMonotonicStream(s *samplestream.SampleStream[uint64], startIdx int, numSamples int, sampleTimeout time.Duration) ([]uint64, error) {
	return verifyMonotonicStreamFunc(s, func(v uint64) uint64 { return v }, startIdx, numSamples, sampleTimeout)
}

// verifyMonotonicStreamFunc awaits numSamples consecutive strictly increasing uint64 counter values
// extracted via extract(T) from s.All()[startIdx:] by polling every 500ms up to sampleTimeout per sample increase.
func verifyMonotonicStreamFunc[T any](s *samplestream.SampleStream[T], extract func(T) uint64, startIdx int, numSamples int, sampleTimeout time.Duration) ([]uint64, error) {
	if s == nil {
		return nil, fmt.Errorf("sample stream is nil")
	}
	samples := make([]uint64, 0, numSamples)
	var prev uint64
	idx := startIdx
	if idx < 0 {
		idx = 0
	}
	deadline := time.Now().Add(sampleTimeout)
	for len(samples) < numSamples {
		all := s.All()
		progressed := false
		for idx < len(all) {
			v := all[idx]
			idx++
			raw, ok := v.Val()
			if !ok {
				continue
			}
			cur := extract(raw)
			if len(samples) == 0 {
				if cur > 0 {
					samples = append(samples, cur)
					prev = cur
					progressed = true
					break
				}
			} else if cur > prev {
				samples = append(samples, cur)
				prev = cur
				progressed = true
				break
			}
		}
		if len(samples) >= numSamples {
			return samples, nil
		}
		if progressed {
			deadline = time.Now().Add(sampleTimeout)
			continue
		}
		if time.Now().After(deadline) {
			return samples, fmt.Errorf("sample %d/%d did not monotonically increase above %d within %v", len(samples)+1, numSamples, prev, sampleTimeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return samples, nil
}

// buildTest6EncapFlows constructs the 2 concurrent encapsulated traffic flows on OTG port1 as specified in Test-6 Step 2:
//   - Flow 1 (ip4_encap_stream): IPv4 -> 138.0.11.8, DSCP 10, 512B frame, 1000 pps
//   - Flow 2 (ip6_encap_stream): IPv6 -> 2015:aa8::1, DSCP 10, 512B frame, 1000 pps
func buildTest6EncapFlows(pktCount uint32) []gosnappi.Flow {
	cfg := gosnappi.NewConfig()

	addUDP := func(flow gosnappi.Flow) {
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValues(randRange(50001, 10000))
		udp.DstPort().SetValues(randRange(50001, 10000))
	}

	// Flow 1: IPv4 Encapsulated Traffic (ip4_encap_stream).
	f1 := cfg.Flows().Add().SetName("ip4_encap_stream")
	f1.Metrics().SetEnable(true)
	f1.TxRx().Port().SetTxName(otgSrcPort).SetRxNames(otgDstPorts)
	f1.Size().SetFixed(test6FrameSize)
	f1.Rate().SetPps(test6PPS)
	f1.Duration().FixedPackets().SetPackets(pktCount)
	e1 := f1.Packet().Add().Ethernet()
	e1.Src().SetValue(otgPort1.MAC)
	e1.Dst().SetValue(dutPort1.MAC)
	v4Encap := f1.Packet().Add().Ipv4()
	v4Encap.Src().SetValue(otgPort1.IPv4)
	v4Encap.Dst().SetValue(ipv4FlowIP)
	v4Encap.TimeToLive().SetValue(ttl)
	v4Encap.Priority().Dscp().Phb().SetValue(dscpEncapA1)
	addUDP(f1)

	// Flow 2: IPv6 Encapsulated Traffic (ip6_encap_stream).
	f2 := cfg.Flows().Add().SetName("ip6_encap_stream")
	f2.Metrics().SetEnable(true)
	f2.TxRx().Port().SetTxName(otgSrcPort).SetRxNames(otgDstPorts)
	f2.Size().SetFixed(test6FrameSize)
	f2.Rate().SetPps(test6PPS)
	f2.Duration().FixedPackets().SetPackets(pktCount)
	e2 := f2.Packet().Add().Ethernet()
	e2.Src().SetValue(otgPort1.MAC)
	e2.Dst().SetValue(dutPort1.MAC)
	v6Encap := f2.Packet().Add().Ipv6()
	v6Encap.Src().SetValue(otgPort1.IPv6)
	v6Encap.Dst().SetValue(ipv6FlowIP)
	v6Encap.HopLimit().SetValue(ttl)
	v6Encap.TrafficClass().SetValue(dscpEncapA1 << 2)
	addUDP(f2)

	return []gosnappi.Flow{f1, f2}
}

// buildTest6NativeIPv6FallbackFlow constructs the native IPv6 fallback traffic flow on OTG port1
// matching ENCAP_TE_VRF_A ::/0 fallback route -> NHG#3000 -> NHG#3002 -> port2 as specified in Test-6 Step 4.
func buildTest6NativeIPv6FallbackFlow(pktCount uint32) gosnappi.Flow {
	cfg := gosnappi.NewConfig()
	f4 := cfg.Flows().Add().SetName("ip6_native_fallback")
	f4.Metrics().SetEnable(true)
	f4.TxRx().Port().SetTxName(otgSrcPort).SetRxNames([]string{"port2"})
	f4.Size().SetFixed(test6FrameSize)
	f4.Rate().SetPps(test6PPS)
	f4.Duration().FixedPackets().SetPackets(pktCount)
	e4 := f4.Packet().Add().Ethernet()
	e4.Src().SetValue(otgPort1.MAC)
	e4.Dst().SetValue(dutPort1.MAC)
	v6Native := f4.Packet().Add().Ipv6()
	v6Native.Src().SetValue(otgPort1.IPv6)
	v6Native.Dst().SetValue(nativeFallbackIPv6FlowIP)
	v6Native.HopLimit().SetValue(ttl)
	v6Native.TrafficClass().SetValue(dscpEncapA1 << 2)
	udp := f4.Packet().Add().Udp()
	udp.SrcPort().SetValues(randRange(50001, 10000))
	udp.DstPort().SetValues(randRange(50001, 10000))
	return f4
}

// verifyWithinTolerance checks that got is within +/- tolPct of want.
func verifyWithinTolerance(t *testing.T, desc string, got, want uint64, tolPct float64) {
	t.Helper()
	diff := math.Abs(float64(got) - float64(want))
	maxDiff := float64(want) * tolPct
	if diff > maxDiff {
		t.Errorf("%s: got %d, want %d (+/- %.1f%% = %.0f)", desc, got, want, tolPct*100, maxDiff)
	} else {
		t.Logf("PASS %s: got %d, want %d (diff %.0f <= %.0f)", desc, got, want, diff, maxDiff)
	}
}

// verifyOctetsWithinTolerance checks that got is within +/- tolPct of either wantL3 (L3 packet size) or wantL2 (L2 frame size).
func verifyOctetsWithinTolerance(t *testing.T, desc string, got, wantL3, wantL2 uint64, tolPct float64) {
	t.Helper()
	diffL3 := math.Abs(float64(got) - float64(wantL3))
	maxDiffL3 := float64(wantL3) * tolPct
	diffL2 := math.Abs(float64(got) - float64(wantL2))
	maxDiffL2 := float64(wantL2) * tolPct
	if diffL3 <= maxDiffL3 {
		t.Logf("PASS %s (L3 octets): got %d, want %d (diff %.0f <= %.0f)", desc, got, wantL3, diffL3, maxDiffL3)
		return
	}
	if diffL2 <= maxDiffL2 {
		t.Logf("PASS %s (L2 frame octets): got %d, want %d (diff %.0f <= %.0f)", desc, got, wantL2, diffL2, maxDiffL2)
		return
	}
	t.Errorf("%s: got %d, want %d (L3) or %d (L2) (+/- %.1f%%)", desc, got, wantL3, wantL2, tolPct*100)
}

// testSimultaneousEncapAndSubintfStreamingCounters implements Test-6:
// Simultaneous IP-in-IP Encapsulation Tunnel and Subinterface Egress Streaming Telemetry Counters Validation.
func testSimultaneousEncapAndSubintfStreamingCounters(t *testing.T, args *testArgs) {
	dut := args.dut
	ate := args.ate
	otgDev := ate.OTG()
	defaultNI := deviations.DefaultNetworkInstance(dut)

	// =========================================================================
	// Test-6 Step 1: Establish Concurrent gNMI SAMPLE Subscriptions & Record Baseline Counters
	// =========================================================================
	t.Log("Test-6 Step 1: Establishing concurrent gNMI SAMPLE subscriptions (10s interval) for AFT next-hop, AFT prefix, subinterface IPv4/IPv6, and physical interface counters")

	nh201Key := resolveAFTNextHopIndex(t, dut, nh201ID, tunnelDstIP1)
	nh202Key := resolveAFTNextHopIndex(t, dut, nh202ID, tunnelDstIP2)
	t.Logf("Resolved AFT NextHop keys in %s: NH#201 -> %d, NH#202 -> %d", defaultNI, nh201Key, nh202Key)

	v4Pfx := cidr(ipv4EntryPrefix, ipv4EntryPrefixLen)
	v6Pfx := cidr(ipv6EntryPrefix, ipv6EntryPrefixLen)

	// Probe AFT NextHop and AFT Prefix state containers and check whether counter leaves are exported.
	var aftNHCountersSupported, aftPfxCountersSupported bool
	if dut.Vendor() != ondatra.NOKIA {
		nh201Val, nh201OK := safeLookup(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHop(nh201Key).State())
		nh202Val, nh202OK := safeLookup(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHop(nh202Key).State())
		for _, nhCheck := range []struct {
			name    string
			val     *oc.NetworkInstance_Afts_NextHop
			ok      bool
			wantDst string
		}{
			{name: "NH#201", val: nh201Val, ok: nh201OK, wantDst: tunnelDstIP1},
			{name: "NH#202", val: nh202Val, ok: nh202OK, wantDst: tunnelDstIP2},
		} {
			if nhCheck.ok && nhCheck.val != nil {
				if gotHdr := nhCheck.val.GetEncapsulateHeader(); gotHdr != oc.Aft_EncapsulationHeaderType_UNSET && gotHdr != oc.Aft_EncapsulationHeaderType_IPV4 {
					t.Errorf("Test-6 Step 1: %s encapsulate-header: got %v, want %v", nhCheck.name, gotHdr, oc.Aft_EncapsulationHeaderType_IPV4)
				}
				if ipip := nhCheck.val.GetIpInIp(); ipip != nil {
					if ipip.GetSrcIp() != "" && ipip.GetSrcIp() != ipv4OuterSrc111 {
						t.Errorf("Test-6 Step 1: %s ip-in-ip/src-ip: got %q, want %q", nhCheck.name, ipip.GetSrcIp(), ipv4OuterSrc111)
					}
					if ipip.GetDstIp() != "" && ipip.GetDstIp() != nhCheck.wantDst {
						t.Errorf("Test-6 Step 1: %s ip-in-ip/dst-ip: got %q, want %q", nhCheck.name, ipip.GetDstIp(), nhCheck.wantDst)
					}
				}
			}
		}
		aftNHCountersSupported = nh201OK && nh202OK && nh201Val != nil && nh202Val != nil && nh201Val.GetCounters() != nil && nh201Val.GetCounters().PacketsForwarded != nil && nh202Val.GetCounters() != nil && nh202Val.GetCounters().PacketsForwarded != nil
	}
	v4PfxVal, v4PfxOK := safeLookup(t, dut, gnmi.OC().NetworkInstance(vrfEncapA).Afts().Ipv4Entry(v4Pfx).State())
	v6PfxVal, v6PfxOK := safeLookup(t, dut, gnmi.OC().NetworkInstance(vrfEncapA).Afts().Ipv6Entry(v6Pfx).State())
	aftPfxCountersSupported = v4PfxOK && v6PfxOK && v4PfxVal != nil && v6PfxVal != nil && v4PfxVal.GetCounters() != nil && v4PfxVal.GetCounters().PacketsForwarded != nil && v6PfxVal.GetCounters() != nil && v6PfxVal.GetCounters().PacketsForwarded != nil
	t.Logf("AFT Counter telemetry availability on %s (%s): NextHop counters=%v, Prefix counters=%v", dut.Name(), dut.Model(), aftNHCountersSupported, aftPfxCountersSupported)

	subintfCountersSupported := !deviations.SubinterfacePacketCountersMissing(dut) && !deviations.DefaultSubinterfacePacketCountersMissing(dut)
	t.Logf("Subinterface(0) IPv4/IPv6 Counter telemetry availability on %s (%s): %v", dut.Name(), dut.Model(), subintfCountersSupported)

	aftStreams := &aftCounterStreams{
		nh201Key: nh201Key,
		nh202Key: nh202Key,
	}
	if aftNHCountersSupported {
		aftStreams.nh201 = samplestream.New(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHop(nh201Key).State(), test6SampleInterval)
		aftStreams.nh202 = samplestream.New(t, dut, gnmi.OC().NetworkInstance(defaultNI).Afts().NextHop(nh202Key).State(), test6SampleInterval)
	}
	if aftPfxCountersSupported {
		aftStreams.v4Prefix = samplestream.New(t, dut, gnmi.OC().NetworkInstance(vrfEncapA).Afts().Ipv4Entry(v4Pfx).State(), test6SampleInterval)
		aftStreams.v6Prefix = samplestream.New(t, dut, gnmi.OC().NetworkInstance(vrfEncapA).Afts().Ipv6Entry(v6Pfx).State(), test6SampleInterval)
	}
	t.Cleanup(func() {
		if aftStreams.nh201 != nil {
			aftStreams.nh201.Close()
		}
		if aftStreams.nh202 != nil {
			aftStreams.nh202.Close()
		}
		if aftStreams.v4Prefix != nil {
			aftStreams.v4Prefix.Close()
		}
		if aftStreams.v6Prefix != nil {
			aftStreams.v6Prefix.Close()
		}
	})

	portIDs := []string{"port1", "port2", "port3", "port4", "port5"}
	portStreams := make(map[string]*portCounterStreams, len(portIDs))
	for _, pid := range portIDs {
		pName := dut.Port(t, pid).Name()
		ps := &portCounterStreams{
			portID:   pid,
			portName: pName,
		}
		if pid == "port1" {
			ps.physInPkts = samplestream.New(t, dut, gnmi.OC().Interface(pName).Counters().InPkts().State(), test6SampleInterval)
			ps.physInOctets = samplestream.New(t, dut, gnmi.OC().Interface(pName).Counters().InOctets().State(), test6SampleInterval)
			if subintfCountersSupported {
				sub := gnmi.OC().Interface(pName).Subinterface(0)
				ps.v4InPkts = samplestream.New(t, dut, sub.Ipv4().Counters().InPkts().State(), test6SampleInterval)
				ps.v4InFwdOctets = samplestream.New(t, dut, sub.Ipv4().Counters().InForwardedOctets().State(), test6SampleInterval)
				ps.v6InPkts = samplestream.New(t, dut, sub.Ipv6().Counters().InPkts().State(), test6SampleInterval)
				ps.v6InFwdOctets = samplestream.New(t, dut, sub.Ipv6().Counters().InForwardedOctets().State(), test6SampleInterval)
			}
		} else {
			ps.physOutPkts = samplestream.New(t, dut, gnmi.OC().Interface(pName).Counters().OutPkts().State(), test6SampleInterval)
			ps.physOutOctets = samplestream.New(t, dut, gnmi.OC().Interface(pName).Counters().OutOctets().State(), test6SampleInterval)
			if subintfCountersSupported {
				sub := gnmi.OC().Interface(pName).Subinterface(0)
				ps.v4OutPkts = samplestream.New(t, dut, sub.Ipv4().Counters().OutPkts().State(), test6SampleInterval)
				ps.v4OutFwdOctets = samplestream.New(t, dut, sub.Ipv4().Counters().OutForwardedOctets().State(), test6SampleInterval)
				if pid == "port2" {
					ps.v6OutPkts = samplestream.New(t, dut, sub.Ipv6().Counters().OutPkts().State(), test6SampleInterval)
					ps.v6OutFwdOctets = samplestream.New(t, dut, sub.Ipv6().Counters().OutForwardedOctets().State(), test6SampleInterval)
				}
			}
		}
		portStreams[pid] = ps
	}
	t.Cleanup(func() {
		closeStream := func(s *samplestream.SampleStream[uint64]) {
			if s != nil {
				s.Close()
			}
		}
		for _, ps := range portStreams {
			closeStream(ps.physInPkts)
			closeStream(ps.physInOctets)
			closeStream(ps.physOutPkts)
			closeStream(ps.physOutOctets)
			closeStream(ps.v4InPkts)
			closeStream(ps.v4InFwdOctets)
			closeStream(ps.v4OutPkts)
			closeStream(ps.v4OutFwdOctets)
			closeStream(ps.v6InPkts)
			closeStream(ps.v6InFwdOctets)
			closeStream(ps.v6OutPkts)
			closeStream(ps.v6OutFwdOctets)
		}
	})

	snapshotAllCounters := func() (aftCounterValues, map[string]portCounterValues) {
		var aftSnap aftCounterValues
		if aftNHCountersSupported {
			aftSnap.nh201Pkts = readLatestSampleFunc(t, aftStreams.nh201, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetPacketsForwarded() }, "AFT NH#201 PacketsForwarded")
			aftSnap.nh201Octets = readLatestSampleFunc(t, aftStreams.nh201, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetOctetsForwarded() }, "AFT NH#201 OctetsForwarded")
			aftSnap.nh202Pkts = readLatestSampleFunc(t, aftStreams.nh202, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetPacketsForwarded() }, "AFT NH#202 PacketsForwarded")
			aftSnap.nh202Octets = readLatestSampleFunc(t, aftStreams.nh202, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetOctetsForwarded() }, "AFT NH#202 OctetsForwarded")
		}
		if aftPfxCountersSupported {
			aftSnap.v4PrefixPkts = readLatestSampleFunc(t, aftStreams.v4Prefix, func(v *oc.NetworkInstance_Afts_Ipv4Entry) uint64 { return v.GetCounters().GetPacketsForwarded() }, "AFT IPv4 138.0.11.0/24 PacketsForwarded")
			aftSnap.v4PrefixOctets = readLatestSampleFunc(t, aftStreams.v4Prefix, func(v *oc.NetworkInstance_Afts_Ipv4Entry) uint64 { return v.GetCounters().GetOctetsForwarded() }, "AFT IPv4 138.0.11.0/24 OctetsForwarded")
			aftSnap.v6PrefixPkts = readLatestSampleFunc(t, aftStreams.v6Prefix, func(v *oc.NetworkInstance_Afts_Ipv6Entry) uint64 { return v.GetCounters().GetPacketsForwarded() }, "AFT IPv6 2015:aa8::/64 PacketsForwarded")
			aftSnap.v6PrefixOctets = readLatestSampleFunc(t, aftStreams.v6Prefix, func(v *oc.NetworkInstance_Afts_Ipv6Entry) uint64 { return v.GetCounters().GetOctetsForwarded() }, "AFT IPv6 2015:aa8::/64 OctetsForwarded")
		}
		portSnap := make(map[string]portCounterValues, len(portIDs))
		for _, pid := range portIDs {
			ps := portStreams[pid]
			var pv portCounterValues
			if pid == "port1" {
				pv.physInPkts = readLatestSample(t, ps.physInPkts, pid+" physInPkts")
				pv.physInOctets = readLatestSample(t, ps.physInOctets, pid+" physInOctets")
				if subintfCountersSupported {
					pv.v4InPkts = readLatestSample(t, ps.v4InPkts, pid+" subintf0 v4InPkts")
					pv.v4InFwdOctets = readLatestSample(t, ps.v4InFwdOctets, pid+" subintf0 v4InFwdOctets")
					pv.v6InPkts = readLatestSample(t, ps.v6InPkts, pid+" subintf0 v6InPkts")
					pv.v6InFwdOctets = readLatestSample(t, ps.v6InFwdOctets, pid+" subintf0 v6InFwdOctets")
				}
			} else {
				pv.physOutPkts = readLatestSample(t, ps.physOutPkts, pid+" physOutPkts")
				pv.physOutOctets = readLatestSample(t, ps.physOutOctets, pid+" physOutOctets")
				if subintfCountersSupported {
					pv.v4OutPkts = readLatestSample(t, ps.v4OutPkts, pid+" subintf0 v4OutPkts")
					pv.v4OutFwdOctets = readLatestSample(t, ps.v4OutFwdOctets, pid+" subintf0 v4OutFwdOctets")
					if pid == "port2" {
						pv.v6OutPkts = readLatestSample(t, ps.v6OutPkts, pid+" subintf0 v6OutPkts")
						pv.v6OutFwdOctets = readLatestSample(t, ps.v6OutFwdOctets, pid+" subintf0 v6OutFwdOctets")
					}
				}
			}
			portSnap[pid] = pv
		}
		return aftSnap, portSnap
	}

	advanceAllStreams := func() {
		var settleWG sync.WaitGroup
		advanceStream := func(s *samplestream.SampleStream[uint64]) {
			if s == nil {
				return
			}
			settleWG.Add(1)
			go func() {
				defer settleWG.Done()
				s.Next()
			}()
		}
		if aftStreams.nh201 != nil {
			settleWG.Add(1)
			go func() { defer settleWG.Done(); aftStreams.nh201.Next() }()
		}
		if aftStreams.nh202 != nil {
			settleWG.Add(1)
			go func() { defer settleWG.Done(); aftStreams.nh202.Next() }()
		}
		if aftStreams.v4Prefix != nil {
			settleWG.Add(1)
			go func() { defer settleWG.Done(); aftStreams.v4Prefix.Next() }()
		}
		if aftStreams.v6Prefix != nil {
			settleWG.Add(1)
			go func() { defer settleWG.Done(); aftStreams.v6Prefix.Next() }()
		}
		for _, ps := range portStreams {
			advanceStream(ps.physInPkts)
			advanceStream(ps.physInOctets)
			advanceStream(ps.physOutPkts)
			advanceStream(ps.physOutOctets)
			if subintfCountersSupported {
				advanceStream(ps.v4InPkts)
				advanceStream(ps.v4InFwdOctets)
				advanceStream(ps.v4OutPkts)
				advanceStream(ps.v4OutFwdOctets)
				advanceStream(ps.v6InPkts)
				advanceStream(ps.v6InFwdOctets)
				advanceStream(ps.v6OutPkts)
				advanceStream(ps.v6OutFwdOctets)
			}
		}
		settleWG.Wait()
	}

	// Wait one parallel sample interval so all active SAMPLE streams have delivered their initial baseline sample (C_before).
	advanceAllStreams()
	_, _ = snapshotAllCounters()

	// =========================================================================
	// Test-6 Step 2: Sustained Encapsulated Traffic Transmission (4in4 and 6in4)
	// =========================================================================
	pktCount := test6PacketCount(dut)
	t.Logf("Test-6 Step 2: Starting sustained dual-stack encapsulated flows on OTG port1 (ip4_encap_stream and ip6_encap_stream, %d packets each)", pktCount)
	encapFlows := buildTest6EncapFlows(pktCount)
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(encapFlows...)
	otgDev.PushConfig(t, args.topo)
	otgDev.StartProtocols(t)
	otgutils.WaitForARP(t, otgDev, args.topo, "IPv4")
	otgutils.WaitForARP(t, otgDev, args.topo, "IPv6")

	// Refresh baseline snapshot after OTG protocol/ARP setup and just before traffic transmission.
	baseAFT, basePorts := snapshotAllCounters()

	// Include the latest pre-traffic sample index (len-1) so S_1 < S_2 < S_3 captures the baseline-to-active transition and >= 2 in-flight increments.
	startIdx := func(n int) int {
		if n > 0 {
			return n - 1
		}
		return 0
	}

	type monotonicCheck struct {
		name string
		run  func() ([]uint64, error)
	}
	var checks []monotonicCheck
	if aftNHCountersSupported {
		nh201Idx := startIdx(len(aftStreams.nh201.All()))
		nh202Idx := startIdx(len(aftStreams.nh202.All()))
		checks = append(checks,
			monotonicCheck{
				name: "AFT NH#201 PacketsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.nh201, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetPacketsForwarded() }, nh201Idx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT NH#201 OctetsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.nh201, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetOctetsForwarded() }, nh201Idx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT NH#202 PacketsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.nh202, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetPacketsForwarded() }, nh202Idx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT NH#202 OctetsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.nh202, func(v *oc.NetworkInstance_Afts_NextHop) uint64 { return v.GetCounters().GetOctetsForwarded() }, nh202Idx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		)
	}
	if aftPfxCountersSupported {
		v4PfxIdx := startIdx(len(aftStreams.v4Prefix.All()))
		v6PfxIdx := startIdx(len(aftStreams.v6Prefix.All()))
		checks = append(checks,
			monotonicCheck{
				name: "AFT IPv4 138.0.11.0/24 PacketsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.v4Prefix, func(v *oc.NetworkInstance_Afts_Ipv4Entry) uint64 { return v.GetCounters().GetPacketsForwarded() }, v4PfxIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT IPv4 138.0.11.0/24 OctetsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.v4Prefix, func(v *oc.NetworkInstance_Afts_Ipv4Entry) uint64 { return v.GetCounters().GetOctetsForwarded() }, v4PfxIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT IPv6 2015:aa8::/64 PacketsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.v6Prefix, func(v *oc.NetworkInstance_Afts_Ipv6Entry) uint64 { return v.GetCounters().GetPacketsForwarded() }, v6PfxIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "AFT IPv6 2015:aa8::/64 OctetsForwarded",
				run: func() ([]uint64, error) {
					return verifyMonotonicStreamFunc(aftStreams.v6Prefix, func(v *oc.NetworkInstance_Afts_Ipv6Entry) uint64 { return v.GetCounters().GetOctetsForwarded() }, v6PfxIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		)
	}
	p1InPktsIdx := startIdx(len(portStreams["port1"].physInPkts.All()))
	p1InOctIdx := startIdx(len(portStreams["port1"].physInOctets.All()))
	checks = append(checks,
		monotonicCheck{
			name: "port1 Physical InPkts",
			run: func() ([]uint64, error) {
				return verifyMonotonicStream(portStreams["port1"].physInPkts, p1InPktsIdx, test6MonotonicSamples, test6SampleTimeout)
			},
		},
		monotonicCheck{
			name: "port1 Physical InOctets",
			run: func() ([]uint64, error) {
				return verifyMonotonicStream(portStreams["port1"].physInOctets, p1InOctIdx, test6MonotonicSamples, test6SampleTimeout)
			},
		},
	)
	for _, pid := range otgDstPorts {
		pid := pid
		outPktsIdx := startIdx(len(portStreams[pid].physOutPkts.All()))
		outOctIdx := startIdx(len(portStreams[pid].physOutOctets.All()))
		checks = append(checks,
			monotonicCheck{
				name: pid + " Physical OutPkts",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams[pid].physOutPkts, outPktsIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: pid + " Physical OutOctets",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams[pid].physOutOctets, outOctIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		)
	}
	if subintfCountersSupported {
		p1V4InPktsIdx := startIdx(len(portStreams["port1"].v4InPkts.All()))
		p1V4InOctIdx := startIdx(len(portStreams["port1"].v4InFwdOctets.All()))
		p1V6InPktsIdx := startIdx(len(portStreams["port1"].v6InPkts.All()))
		p1V6InOctIdx := startIdx(len(portStreams["port1"].v6InFwdOctets.All()))
		checks = append(checks,
			monotonicCheck{
				name: "port1 Subinterface(0) IPv4 InPkts",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port1"].v4InPkts, p1V4InPktsIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "port1 Subinterface(0) IPv4 InForwardedOctets",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port1"].v4InFwdOctets, p1V4InOctIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "port1 Subinterface(0) IPv6 InPkts",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port1"].v6InPkts, p1V6InPktsIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			monotonicCheck{
				name: "port1 Subinterface(0) IPv6 InForwardedOctets",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port1"].v6InFwdOctets, p1V6InOctIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		)
		for _, pid := range otgDstPorts {
			pid := pid
			v4OutPktsIdx := startIdx(len(portStreams[pid].v4OutPkts.All()))
			v4OutOctIdx := startIdx(len(portStreams[pid].v4OutFwdOctets.All()))
			checks = append(checks,
				monotonicCheck{
					name: pid + " Subinterface(0) IPv4 OutPkts",
					run: func() ([]uint64, error) {
						return verifyMonotonicStream(portStreams[pid].v4OutPkts, v4OutPktsIdx, test6MonotonicSamples, test6SampleTimeout)
					},
				},
				monotonicCheck{
					name: pid + " Subinterface(0) IPv4 OutForwardedOctets",
					run: func() ([]uint64, error) {
						return verifyMonotonicStream(portStreams[pid].v4OutFwdOctets, v4OutOctIdx, test6MonotonicSamples, test6SampleTimeout)
					},
				},
			)
		}
	}

	otgDev.StartTraffic(t)

	// =========================================================================
	// Test-6 Step 3: Periodic Streaming Telemetry & Simultaneous Feature Coexistence Verification
	// =========================================================================
	t.Log("Test-6 Step 3: Validating real-time monotonic counter progression (S_1 < S_2 < ... < S_k) across >= 3 consecutive 10s gNMI SAMPLE updates while encapsulated traffic is active")

	var wg sync.WaitGroup
	var mu sync.Mutex
	var step3Errors []string
	for _, chk := range checks {
		chk := chk
		wg.Add(1)
		go func() {
			defer wg.Done()
			vals, err := chk.run()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				step3Errors = append(step3Errors, fmt.Sprintf("%s: %v", chk.name, err))
			} else {
				t.Logf("Test-6 Step 3 PASS [%s]: monotonic samples = %v", chk.name, vals)
			}
		}()
	}
	wg.Wait()
	for _, errMsg := range step3Errors {
		t.Errorf("Test-6 Step 3 monotonic check failed: %s", errMsg)
	}

	// Wait via gnmi.Watch for both encapsulated flows to finish transmitting all fixed packets.
	for _, f := range encapFlows {
		gnmi.Watch(t, otgDev, gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State(), 120*time.Second, func(val *ygnmi.Value[uint64]) bool {
			outPkts, present := val.Val()
			return present && outPkts >= uint64(pktCount)
		}).Await(t)
	}
	otgDev.StopTraffic(t)

	otgutils.LogPortMetrics(t, otgDev, args.topo)
	otgutils.LogFlowMetrics(t, otgDev, args.topo)

	// Verify 0% packet loss across both encapsulated flows.
	for _, f := range encapFlows {
		otgutils.ExpectedTrafficLoss(t, otgDev, f.Name(), 0, 1)
	}

	// =========================================================================
	// Test-6 Step 4: Post-Traffic Counter Delta & Hierarchical Distribution Accuracy
	// =========================================================================
	t.Log("Test-6 Step 4: Waiting for post-traffic gNMI SAMPLE interval (C_after) and validating final counter deltas")

	// Wait deterministically for one post-traffic sample on port1 Physical InPkts so that all sample streams have settled.
	wantEncapIngressPkts := basePorts["port1"].physInPkts + 2*uint64(pktCount)
	for i := 0; i < 4; i++ {
		if next := portStreams["port1"].physInPkts.Next(); next != nil {
			if v, ok := next.Val(); ok && v >= uint64(float64(wantEncapIngressPkts)*(1.0-test6CounterTolPct)) {
				break
			}
		}
	}
	// Advance one settled post-traffic sample across all streams.
	advanceAllStreams()
	finalAFT, finalPorts := snapshotAllCounters()

	// Retrieve OTG flow TxPkts and RxPkts (N_tx_v4, N_tx_v6, and N_rx_total = N_tx_v4 + N_tx_v6).
	txFlowV4 := gnmi.Get(t, otgDev, gnmi.OTG().Flow("ip4_encap_stream").Counters().OutPkts().State())
	txFlowV6 := gnmi.Get(t, otgDev, gnmi.OTG().Flow("ip6_encap_stream").Counters().OutPkts().State())
	rxFlowV4 := gnmi.Get(t, otgDev, gnmi.OTG().Flow("ip4_encap_stream").Counters().InPkts().State())
	rxFlowV6 := gnmi.Get(t, otgDev, gnmi.OTG().Flow("ip6_encap_stream").Counters().InPkts().State())
	totalEncapTx := txFlowV4 + txFlowV6
	totalEncapRx := rxFlowV4 + rxFlowV6

	// 1. IP-in-IP Encap Tunnel Next-Hop & AFT Prefix Counters:
	//    - Combined packet delta across NH#201 and NH#202 equals N_tx_v4 + N_tx_v6 (+/- 1%).
	//    - Individual tunnel next-hop packet and octet deltas reflect 1:3 WCMP weights (25% / 75% +/- 5%).
	deltaNH201Pkts := finalAFT.nh201Pkts - baseAFT.nh201Pkts
	deltaNH202Pkts := finalAFT.nh202Pkts - baseAFT.nh202Pkts
	deltaNH201Octets := finalAFT.nh201Octets - baseAFT.nh201Octets
	deltaNH202Octets := finalAFT.nh202Octets - baseAFT.nh202Octets
	totalNHPkts := deltaNH201Pkts + deltaNH202Pkts
	totalNHOctets := deltaNH201Octets + deltaNH202Octets
	if aftNHCountersSupported {
		verifyWithinTolerance(t, "Test-6 Step 4: Combined AFT Encap NextHop PacketsForwarded (delta_NH201_pkts + delta_NH202_pkts)", totalNHPkts, totalEncapTx, test6CounterTolPct)

		if totalNHPkts > 0 {
			gotNH201PktRatio := float64(deltaNH201Pkts) / float64(totalNHPkts)
			gotNH202PktRatio := float64(deltaNH202Pkts) / float64(totalNHPkts)
			if math.Abs(gotNH201PktRatio-ratioTunEncap1) > ratioTunEncapTol {
				t.Errorf("Test-6 Step 4: AFT NH#201 packet WCMP share got %.4f, want %.4f +/- %.4f", gotNH201PktRatio, ratioTunEncap1, ratioTunEncapTol)
			}
			if math.Abs(gotNH202PktRatio-ratioTunEncap2) > ratioTunEncapTol {
				t.Errorf("Test-6 Step 4: AFT NH#202 packet WCMP share got %.4f, want %.4f +/- %.4f", gotNH202PktRatio, ratioTunEncap2, ratioTunEncapTol)
			}
		}
		if totalNHOctets > 0 {
			gotNH201OctetRatio := float64(deltaNH201Octets) / float64(totalNHOctets)
			gotNH202OctetRatio := float64(deltaNH202Octets) / float64(totalNHOctets)
			if math.Abs(gotNH201OctetRatio-ratioTunEncap1) > ratioTunEncapTol {
				t.Errorf("Test-6 Step 4: AFT NH#201 octet WCMP share got %.4f, want %.4f +/- %.4f", gotNH201OctetRatio, ratioTunEncap1, ratioTunEncapTol)
			}
			if math.Abs(gotNH202OctetRatio-ratioTunEncap2) > ratioTunEncapTol {
				t.Errorf("Test-6 Step 4: AFT NH#202 octet WCMP share got %.4f, want %.4f +/- %.4f", gotNH202OctetRatio, ratioTunEncap2, ratioTunEncapTol)
			}
		}
		wantNHTotalInnerOctets := totalNHPkts * test6InnerL3Bytes
		wantNHTotalOuterOctets := totalNHPkts * test6OuterL3Bytes
		if math.Abs(float64(totalNHOctets)-float64(wantNHTotalInnerOctets)) > float64(wantNHTotalInnerOctets)*test6CounterTolPct &&
			math.Abs(float64(totalNHOctets)-float64(wantNHTotalOuterOctets)) > float64(wantNHTotalOuterOctets)*test6CounterTolPct {
			t.Errorf("Test-6 Step 4: Total AFT Encap NextHop OctetsForwarded got %d, want %d (inner L3) or %d (outer L3) +/- 1%%",
				totalNHOctets, wantNHTotalInnerOctets, wantNHTotalOuterOctets)
		}
	}

	if aftPfxCountersSupported {
		deltaV4PfxPkts := finalAFT.v4PrefixPkts - baseAFT.v4PrefixPkts
		deltaV6PfxPkts := finalAFT.v6PrefixPkts - baseAFT.v6PrefixPkts
		verifyWithinTolerance(t, "Test-6 Step 4: AFT ENCAP_TE_VRF_A IPv4 138.0.11.0/24 PacketsForwarded", deltaV4PfxPkts, txFlowV4, test6CounterTolPct)
		verifyWithinTolerance(t, "Test-6 Step 4: AFT ENCAP_TE_VRF_A IPv6 2015:aa8::/64 PacketsForwarded", deltaV6PfxPkts, txFlowV6, test6CounterTolPct)
	}

	// 2. Subinterface and Physical Interface Egress Counters:
	//    - Because both 4in4 and 6in4 encapsulated packets egress the DUT with an outer IPv4 tunnel header,
	//      the sum of egress subinterface IPv4 packet deltas (ipv4/state/counters/out-pkts) across DUT
	//      port-2 through port-5, as well as the sum of physical interface egress packet deltas
	//      (state/counters/out-pkts) across those same ports, each independently equals the total
	//      encapsulated packets received on ATE port-2 through port-5 (N_rx_total = N_tx_v4 + N_tx_v6).
	var sumPhysOutPkts uint64
	var sumPhysOutOctets uint64
	var physOutPktsPerPort []uint64
	for _, pid := range otgDstPorts {
		dPhysOut := finalPorts[pid].physOutPkts - basePorts[pid].physOutPkts
		dPhysOutOct := finalPorts[pid].physOutOctets - basePorts[pid].physOutOctets
		sumPhysOutPkts += dPhysOut
		sumPhysOutOctets += dPhysOutOct
		physOutPktsPerPort = append(physOutPktsPerPort, dPhysOut)
	}
	verifyWithinTolerance(t, "Test-6 Step 4: Independent sum of Physical Interface Egress OutPkts across port2..port5 vs N_rx_total (N_tx_v4 + N_tx_v6)", sumPhysOutPkts, totalEncapRx, test6CounterTolPct)
	verifyOctetsWithinTolerance(t, "Test-6 Step 4: Sum of Physical Interface Egress OutOctets across port2..port5",
		sumPhysOutOctets, totalEncapRx*test6OuterL3Bytes, totalEncapRx*test6OuterL2Bytes, test6CounterTolPct)
	if sumPhysOutPkts == 0 {
		t.Fatalf("Test-6 Step 4: sum of physical interface egress OutPkts is 0, cannot calculate WCMP distribution")
	}
	gotPhysWeights, _ := normalize(physOutPktsPerPort)
	if diff := cmp.Diff(wantWeights, gotPhysWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Test-6 Step 4: Physical Interface Egress WCMP distribution ratios -want,+got:\n%s", diff)
	}

	if subintfCountersSupported {
		var sumEgressSubintfV4OutPkts uint64
		var sumEgressSubintfV4OutOctets uint64
		var encapPktsPerPort []uint64
		for _, pid := range otgDstPorts {
			dV4OutPkts := finalPorts[pid].v4OutPkts - basePorts[pid].v4OutPkts
			dV4OutOctets := finalPorts[pid].v4OutFwdOctets - basePorts[pid].v4OutFwdOctets
			sumEgressSubintfV4OutPkts += dV4OutPkts
			sumEgressSubintfV4OutOctets += dV4OutOctets
			encapPktsPerPort = append(encapPktsPerPort, dV4OutPkts)

			dPhysOut := finalPorts[pid].physOutPkts - basePorts[pid].physOutPkts
			verifyWithinTolerance(t, fmt.Sprintf("Test-6 Step 4: %s Physical OutPkts vs Subinterface(0) IPv4 OutPkts", pid), dPhysOut, dV4OutPkts, test6CounterTolPct)
		}
		verifyWithinTolerance(t, "Test-6 Step 4: Independent sum of Egress Subinterface(0) IPv4 OutPkts across port2..port5 vs N_rx_total (N_tx_v4 + N_tx_v6)", sumEgressSubintfV4OutPkts, totalEncapRx, test6CounterTolPct)

		verifyOctetsWithinTolerance(t, "Test-6 Step 4: Sum of Egress Subinterface(0) IPv4 OutForwardedOctets across port2..port5",
			sumEgressSubintfV4OutOctets, totalEncapRx*test6OuterL3Bytes, totalEncapRx*test6OuterL2Bytes, test6CounterTolPct)

		// Per-port egress subinterface IPv4 counter deltas on port-2, port-3, port-4, and port-5
		// match the expected hierarchical WCMP distribution (6.25%, 18.75%, 30%, and 45% with +/- 2% tolerance).
		if sumEgressSubintfV4OutPkts == 0 {
			t.Fatalf("Test-6 Step 4: sum of egress subinterface IPv4 OutPkts is 0, cannot calculate WCMP distribution")
		}
		gotSubintfWeights, _ := normalize(encapPktsPerPort)
		if diff := cmp.Diff(wantWeights, gotSubintfWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
			t.Errorf("Test-6 Step 4: Egress Subinterface(0) IPv4 WCMP distribution ratios -want,+got:\n%s", diff)
		}

		// 3. Ingress Subinterface Counters on DUT port-1:
		//    - ipv4/state/counters/in-pkts and in-forwarded-octets equal N_tx_v4 packets and octets.
		//    - ipv6/state/counters/in-pkts and in-forwarded-octets equal N_tx_v6 packets and octets.
		p1DeltaV4InPkts := finalPorts["port1"].v4InPkts - basePorts["port1"].v4InPkts
		p1DeltaV4InOctets := finalPorts["port1"].v4InFwdOctets - basePorts["port1"].v4InFwdOctets
		p1DeltaV6InPkts := finalPorts["port1"].v6InPkts - basePorts["port1"].v6InPkts
		p1DeltaV6InOctets := finalPorts["port1"].v6InFwdOctets - basePorts["port1"].v6InFwdOctets

		verifyWithinTolerance(t, "Test-6 Step 4: port1 Subinterface(0) IPv4 InPkts (N_tx_v4)", p1DeltaV4InPkts, txFlowV4, test6CounterTolPct)
		verifyOctetsWithinTolerance(t, "Test-6 Step 4: port1 Subinterface(0) IPv4 InForwardedOctets", p1DeltaV4InOctets, txFlowV4*test6InnerL3Bytes, txFlowV4*test6InnerL2Bytes, test6CounterTolPct)
		verifyWithinTolerance(t, "Test-6 Step 4: port1 Subinterface(0) IPv6 InPkts (N_tx_v6)", p1DeltaV6InPkts, txFlowV6, test6CounterTolPct)
		verifyOctetsWithinTolerance(t, "Test-6 Step 4: port1 Subinterface(0) IPv6 InForwardedOctets", p1DeltaV6InOctets, txFlowV6*test6InnerL3Bytes, txFlowV6*test6InnerL2Bytes, test6CounterTolPct)
	}

	p1DeltaPhysInPkts := finalPorts["port1"].physInPkts - basePorts["port1"].physInPkts
	verifyWithinTolerance(t, "Test-6 Step 4: port1 Physical InPkts (N_tx_v4 + N_tx_v6)", p1DeltaPhysInPkts, totalEncapTx, test6CounterTolPct)

	// 4. Post-Traffic Stability Verification:
	//    - Subsequent streaming sample updates after traffic stops remain stable with no stale backlog or unexpected counter drift.
	t.Log("Test-6 Step 4: Collecting subsequent post-traffic gNMI SAMPLE update to verify counter stability (no stale backlog or drift)")
	advanceAllStreams()
	stableAFT, stablePorts := snapshotAllCounters()
	if aftNHCountersSupported {
		verifyWithinTolerance(t, "Test-6 Step 4 Stability: AFT NH#201 + NH#202 PacketsForwarded post-traffic stability",
			(stableAFT.nh201Pkts-baseAFT.nh201Pkts)+(stableAFT.nh202Pkts-baseAFT.nh202Pkts), totalNHPkts, test6CounterTolPct)
	}
	var stableSumPhysOutPkts uint64
	for _, pid := range otgDstPorts {
		stableSumPhysOutPkts += stablePorts[pid].physOutPkts - basePorts[pid].physOutPkts
	}
	verifyWithinTolerance(t, "Test-6 Step 4 Stability: Physical Interface Egress OutPkts post-traffic stability",
		stableSumPhysOutPkts, totalEncapRx, test6CounterTolPct)
	if subintfCountersSupported {
		var stableSumSubintfV4OutPkts uint64
		for _, pid := range otgDstPorts {
			stableSumSubintfV4OutPkts += stablePorts[pid].v4OutPkts - basePorts[pid].v4OutPkts
		}
		verifyWithinTolerance(t, "Test-6 Step 4 Stability: Egress Subinterface(0) IPv4 OutPkts post-traffic stability",
			stableSumSubintfV4OutPkts, totalEncapRx, test6CounterTolPct)
	}

	// 5. Native IPv6 Egress Validation (via ::/0 fallback route in ENCAP_TE_VRF_A -> NHG#3000 -> NHG#3002 -> port-2):
	//    - Verify /interfaces/interface[name=port-2]/subinterfaces/subinterface[index=0]/ipv6/state/counters/out-pkts
	//      and out-forwarded-octets (or physical interface out-pkts/out-octets when subinterface counters are unsupported)
	//      increment monotonically via SAMPLE streaming updates and match the native IPv6 packets and octets received on ATE port-2.
	t.Log("Test-6 Step 4: Validating native IPv6 egress counters on port2 via ::/0 fallback route in ENCAP_TE_VRF_A")
	baseP2PhysOutPkts := stablePorts["port2"].physOutPkts
	baseP2PhysOutOctets := stablePorts["port2"].physOutOctets
	baseP2V6OutPkts := stablePorts["port2"].v6OutPkts
	baseP2V6OutOctets := stablePorts["port2"].v6OutFwdOctets

	nativeV6Flow := buildTest6NativeIPv6FallbackFlow(pktCount)
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(nativeV6Flow)
	otgDev.PushConfig(t, args.topo)
	otgDev.StartProtocols(t)
	otgutils.WaitForARP(t, otgDev, args.topo, "IPv6")

	var v6Checks []monotonicCheck
	if subintfCountersSupported {
		p2V6OutPktsIdx := startIdx(len(portStreams["port2"].v6OutPkts.All()))
		p2V6OutOctIdx := startIdx(len(portStreams["port2"].v6OutFwdOctets.All()))
		v6Checks = []monotonicCheck{
			{
				name: "port2 Subinterface(0) IPv6 OutPkts (native fallback)",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port2"].v6OutPkts, p2V6OutPktsIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			{
				name: "port2 Subinterface(0) IPv6 OutForwardedOctets (native fallback)",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port2"].v6OutFwdOctets, p2V6OutOctIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		}
	} else {
		p2PhysOutPktsIdx := startIdx(len(portStreams["port2"].physOutPkts.All()))
		p2PhysOutOctIdx := startIdx(len(portStreams["port2"].physOutOctets.All()))
		v6Checks = []monotonicCheck{
			{
				name: "port2 Physical OutPkts (native IPv6 fallback)",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port2"].physOutPkts, p2PhysOutPktsIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
			{
				name: "port2 Physical OutOctets (native IPv6 fallback)",
				run: func() ([]uint64, error) {
					return verifyMonotonicStream(portStreams["port2"].physOutOctets, p2PhysOutOctIdx, test6MonotonicSamples, test6SampleTimeout)
				},
			},
		}
	}

	otgDev.StartTraffic(t)

	var v6WG sync.WaitGroup
	var v6Mu sync.Mutex
	var v6StepErrors []string
	for _, v6Chk := range v6Checks {
		v6Chk := v6Chk
		v6WG.Add(1)
		go func() {
			defer v6WG.Done()
			vals, err := v6Chk.run()
			v6Mu.Lock()
			defer v6Mu.Unlock()
			if err != nil {
				v6StepErrors = append(v6StepErrors, fmt.Sprintf("%s: %v", v6Chk.name, err))
			} else {
				t.Logf("Test-6 Step 4 PASS [%s]: monotonic samples = %v", v6Chk.name, vals)
			}
		}()
	}
	v6WG.Wait()
	for _, errMsg := range v6StepErrors {
		t.Errorf("Test-6 Step 4 native IPv6 monotonic check failed: %s", errMsg)
	}

	gnmi.Watch(t, otgDev, gnmi.OTG().Flow(nativeV6Flow.Name()).Counters().OutPkts().State(), 120*time.Second, func(val *ygnmi.Value[uint64]) bool {
		outPkts, present := val.Val()
		return present && outPkts >= uint64(pktCount)
	}).Await(t)
	otgDev.StopTraffic(t)
	otgutils.ExpectedTrafficLoss(t, otgDev, nativeV6Flow.Name(), 0, 1)

	rxNativeV6 := gnmi.Get(t, otgDev, gnmi.OTG().Flow(nativeV6Flow.Name()).Counters().InPkts().State())
	if subintfCountersSupported {
		wantP2V6OutPkts := baseP2V6OutPkts + rxNativeV6
		for i := 0; i < 4; i++ {
			if next := portStreams["port2"].v6OutPkts.Next(); next != nil {
				if v, ok := next.Val(); ok && v >= uint64(float64(wantP2V6OutPkts)*(1.0-test6CounterTolPct)) {
					break
				}
			}
		}
		portStreams["port2"].v6OutFwdOctets.Next()

		finalP2V6OutPkts := readLatestSample(t, portStreams["port2"].v6OutPkts, "port2 subintf0 v6OutPkts")
		finalP2V6OutOctets := readLatestSample(t, portStreams["port2"].v6OutFwdOctets, "port2 subintf0 v6OutFwdOctets")
		verifyWithinTolerance(t, "Test-6 Step 4: port2 Subinterface(0) IPv6 OutPkts (native IPv6 fallback)", finalP2V6OutPkts-baseP2V6OutPkts, rxNativeV6, test6CounterTolPct)
		verifyOctetsWithinTolerance(t, "Test-6 Step 4: port2 Subinterface(0) IPv6 OutForwardedOctets (native IPv6 fallback)", finalP2V6OutOctets-baseP2V6OutOctets, rxNativeV6*test6InnerL3Bytes, rxNativeV6*test6InnerL2Bytes, test6CounterTolPct)
	} else {
		wantP2PhysOutPkts := baseP2PhysOutPkts + rxNativeV6
		for i := 0; i < 4; i++ {
			if next := portStreams["port2"].physOutPkts.Next(); next != nil {
				if v, ok := next.Val(); ok && v >= uint64(float64(wantP2PhysOutPkts)*(1.0-test6CounterTolPct)) {
					break
				}
			}
		}
		portStreams["port2"].physOutOctets.Next()

		finalP2PhysOutPkts := readLatestSample(t, portStreams["port2"].physOutPkts, "port2 physOutPkts")
		finalP2PhysOutOctets := readLatestSample(t, portStreams["port2"].physOutOctets, "port2 physOutOctets")
		verifyWithinTolerance(t, "Test-6 Step 4: port2 Physical OutPkts (native IPv6 fallback)", finalP2PhysOutPkts-baseP2PhysOutPkts, rxNativeV6, test6CounterTolPct)
		verifyOctetsWithinTolerance(t, "Test-6 Step 4: port2 Physical OutOctets (native IPv6 fallback)", finalP2PhysOutOctets-baseP2PhysOutOctets, rxNativeV6*test6InnerL3Bytes, rxNativeV6*test6InnerL2Bytes, test6CounterTolPct)
	}
}
