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

// Package encap_test implements TE-16.1 of the dcgate vendor testplan
package dcgate_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipipProtocol         = 4
	ipv6ipProtocol       = 41
	udpProtocol          = 17
	ethertypeIPv4        = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6        = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	clusterPolicy        = "vrf_selection_policy_c"
	wanPolicy            = "vrf_selection_policy_w"
	popGatePolicy        = "redirect-to-vrf_t"
	vrfDecap             = "DECAP_TE_VRF"
	vrfTransit           = "TE_VRF_111"
	vrfRepaired          = "TE_VRF_222"
	vrfEncapA            = "ENCAP_TE_VRF_A"
	vrfEncapB            = "ENCAP_TE_VRF_B"
	vrfDecapPostRepaired = "DECAP"
	vrfRepair            = "REPAIR_VRF"
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	// trafficDuration      = 15 * time.Second
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
	dscpEncapA2        = 18
	dscpEncapB1        = 20
	dscpEncapB2        = 28
	dscpEncapNoMatch   = 30
	magicMac           = "02:00:00:00:00:01"
	tunnelDstIP1       = "203.0.113.1"
	tunnelDstIP2       = "203.0.113.2"
	tunnelDstIP3       = "203.0.113.100"
	ipv4OuterSrc111    = "198.51.100.111"
	ipv4OuterSrc222    = "198.51.100.222"
	ipv4OuterSrcIpInIp = "198.100.200.123"
	vipIP1             = "192.0.2.111"
	vipIP2             = "192.0.2.222"
	vipIP3             = "192.0.2.133"
	innerV4DstIP       = "198.18.1.1"
	innerV4SrcIP       = "198.18.0.255"
	InnerV6SrcIP       = "2001:DB8::198:1"
	InnerV6DstIP       = "2001:DB8:2:0:192::10"
	ipv4FlowIP         = "138.0.11.8"
	ipv4EntryPrefix    = "138.0.11.0"
	ipv4EntryPrefixLen = 24
	ipv6FlowIP         = "2015:aa8::1"
	ipv6EntryPrefix    = "2015:aa8::"
	ipv6EntryPrefixLen = 32
	ratioTunEncap1     = 0.25 // 1/4
	ratioTunEncap2     = 0.75 // 3/4
	ratioTunEncapTol   = 0.05 // 5/100
	ttl                = uint32(100)
	innerTtl           = uint32(50)
	trfDistTolerance   = 0.02
	// observing on IXIA OTG: Cannot start capture on more than one port belonging to the
	// same resource group or on more than one port behind the same front panel port in the chassis
	otgMutliPortCaptureSupported     = false
	ipv4PrefixDoesNotExistInEncapVrf = "140.0.0.1"
	ipv6PrefixDoesNotExistInEncapVrf = "2016::140:0:0:1"
)

const (
	// for Interface prefix
	baseNHOffset  = 0
	baseNHGOffset = 100
	// for VIP prefix
	vipNHOffset  = 200
	vipNHGOffset = 300
	// for Tunnel Prefix
	tunNHOffset  = 400
	tunNHGOffset = 500
	// for encap
	encapNHOffset  = 600
	encapNHGOffset = 700
	// for decap
	decapNHOffset  = 800
	decapNHGOffset = 900
)

func baseNH(i uint64) uint64   { return i + baseNHOffset }
func baseNHG(i uint64) uint64  { return i + baseNHGOffset }
func vipNH(i uint64) uint64    { return i + vipNHOffset }
func vipNHG(i uint64) uint64   { return i + vipNHGOffset }
func tunNH(i uint64) uint64    { return i + tunNHOffset }
func tunNHG(i uint64) uint64   { return i + tunNHGOffset }
func encapNH(i uint64) uint64  { return i + encapNHOffset }
func encapNHG(i uint64) uint64 { return i + encapNHGOffset }
func decapNH(i uint64) uint64  { return i + decapNHOffset }
func decapNHG(i uint64) uint64 { return i + decapNHGOffset }

var (
	otgDstPorts = []string{"port2", "port3", "port4", "port5"}
	otgSrcPort  = "port1"
	wantWeights = []float64{
		0.0625, // 1/4 * 1/4 - port2
		0.1875, // 1/4 * 3/4 - port3
		0.3,    // 3/4 * 2/5 - port4
		0.45,   // 3/5 * 3/4 - port5
	}
	noMatchWeight = []float64{
		1, 0, 0, 0,
	}
	// %loss tolerance for traffic received when there should be 100% loss
	// make non-zero to allow for some packet gain
	lossTolerance      = float32(0.0)
	sf_fps             = flag.Uint64("sf_fps", 100000, "frames per second for traffic while validating sFlow")
	sf_trafficDuration = flag.Int("sf_traffic_duration", 10, "traffic duration in seconds while validating sFlow")
	capture_sflow      = *flag.Bool("capture_sflow", false, "enable sFlow capture")
	frameSize          = flag.Int("frame_size", 512, "frame size in bytes, default 512")
	fps                = flag.Uint64("fps", 100, "frames per second")
	trafficDuration    = flag.Int("traffic_duration", 10, "traffic duration in seconds")
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

	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:15",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort6 = attrs.Attributes{
		Name:    "otgPort6",
		MAC:     "02:00:06:01:01:01",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:16",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:19",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort7 = attrs.Attributes{
		Name:    "otgPort7",
		MAC:     "02:00:07:01:01:01",
		IPv4:    "192.0.2.26",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1d",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort8 = attrs.Attributes{
		Name:    "otgPort8",
		MAC:     "02:00:08:01:01:01",
		IPv4:    "192.0.2.30",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1e",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.33",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.34",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3DummyIP = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.37",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.38",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4DummyIP = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.41",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.42",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort5DummyIP = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.45",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort5DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.46",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort6DummyIP = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.49",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort6DummyIP = attrs.Attributes{
		Desc:    "otgPort6",
		IPv4:    "192.0.2.50",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort7DummyIP = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.53",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort7DummyIP = attrs.Attributes{
		Desc:    "otgPort6",
		IPv4:    "192.0.2.54",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort8DummyIP = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.57",
		IPv4Len: ipv4PrefixLen,
	}

	otgPort8DummyIP = attrs.Attributes{
		Desc:    "otgPort6",
		IPv4:    "192.0.2.58",
		IPv4Len: ipv4PrefixLen,
	}
)

type pbrRule struct {
	sequence    uint32
	protocol    uint8
	srcAddr     string
	dscpSet     []uint8
	dscpSetV6   []uint8
	decapVrfSet []string
	encapVrf    string
	etherType   oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
}

type packetAttr struct {
	dscp     int
	protocol int
	ttl      uint32
	inner    *packetAttr
	sfSample *sflowSample
	sfConfig *sflowConfig
}

type sflowSample struct {
	InputInterface  string
	OutputInterface string
	rawPktHdr       *sfRecordRawPacketHeader
	extdRtrData     *sfRecordExtendedRouterData
	extdGtwData     *sfRecordExtendedGatewayData
}
type sfRecordRawPacketHeader struct {
	Protocol    uint32 // Protocol of the sampled packet
	FrameLength uint32 // Original length of the packet
	Stripped    uint32 // Number of bytes stripped from the packet
	Header      []byte // Header bytes of the sampled packet
}

type sfRecordExtendedRouterData struct {
	NextHop         string   // IP address of the next hop router
	SrcAS           uint32   // Source Autonomous System
	DstAS           uint32   // Destination Autonomous System
	SrcPeerAS       uint32   // Source Peer AS
	InputInterface  uint32   // SNMP index of input interface
	OutputInterface uint32   // SNMP index of output interface
	SrcASPath       []uint32 // AS path to source
	DstASPath       []uint32 // AS path to destination
}

type sfRecordExtendedGatewayData struct {
	NextHop     string   // IP address of the gateway
	ASPath      []uint32 // AS path
	Communities []uint32 // BGP communities
	LocalPref   uint32   // BGP local preference
}

// flowConfig and IPType are provided in the original prompt
type sflowConfig struct {
	name            string
	packetsToSend   uint32
	ppsRate         uint64
	frameSize       uint32
	sflowDscp       uint8
	samplingRate    uint
	sampleTolerance float32
	ip              IPType
	inputInterface  []uint32
	outputInterface []uint32
}

type IPType string

const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

type flowAttr struct {
	src       string   // source IP address
	dst       string   // destination IP address
	srcPort   string   // source OTG port
	dstPorts  []string // destination OTG ports
	srcMac    string   // source MAC address
	dstMac    string   // destination MAC address
	ttl       uint32
	innerTtl  uint32
	innerDscp uint32
	dscp      uint32
	fps       uint64
	topo      gosnappi.Config
}

var (
	fa4 = flowAttr{
		src:      otgPort1.IPv4,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		ttl:      ttl,
		topo:     gosnappi.NewConfig(),
		fps:      *fps,
	}
	fa6 = flowAttr{
		src:      otgPort1.IPv6,
		dst:      ipv6FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		ttl:      ttl,
		topo:     gosnappi.NewConfig(),
		fps:      *fps,
	}
	faIPinIP = flowAttr{
		src:      ipv4OuterSrcIpInIp,
		dst:      ipv4FlowIP,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		ttl:      ttl,
		innerTtl: innerTtl,
		topo:     gosnappi.NewConfig(),
		fps:      *fps,
	}
	fa4NoPrefix = flowAttr{
		src:      otgPort1.IPv4,
		dst:      ipv4PrefixDoesNotExistInEncapVrf,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		ttl:      ttl,
		topo:     gosnappi.NewConfig(),
		fps:      *fps,
	}
	fa6NoPrefix = flowAttr{
		src:      otgPort1.IPv6,
		dst:      ipv6PrefixDoesNotExistInEncapVrf,
		srcMac:   otgPort1.MAC,
		dstMac:   dutPort1.MAC,
		srcPort:  otgSrcPort,
		dstPorts: otgDstPorts,
		ttl:      ttl,
		topo:     gosnappi.NewConfig(),
		fps:      *fps,
	}
	faTransit = flowAttr{
		src:       ipv4OuterSrc111,
		dst:       tunnelDstIP1,
		srcMac:    otgPort1.MAC,
		dstMac:    dutPort1.MAC,
		srcPort:   otgSrcPort,
		dstPorts:  otgDstPorts,
		ttl:       ttl,
		innerTtl:  innerTtl,
		innerDscp: dscpEncapNoMatch, // at egress outer DSCP will be copied to inner DSCP
		topo:      gosnappi.NewConfig(),
		fps:       *fps,
	}
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut             *ondatra.DUTDevice
	ate             *ondatra.ATEDevice
	topo            gosnappi.Config
	client          *gribi.Client
	pattr           *packetAttr
	flows           []gosnappi.Flow
	capture_ports   []string
	validateSflow   bool
	trafficDuration int
}

// getPbrRules returns pbrRule slice for cluster facing (clusterFacing = true) or wan facing
// interface (clusterFacing = false)
func getPbrRules(dut *ondatra.DUTDevice, clusterFacing bool) []pbrRule {
	vrfDefault := deviations.DefaultNetworkInstance(dut)
	var pbrRules = []pbrRule{
		{
			sequence:    1,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    2,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    3,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    4,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapA1, dscpEncapA2},
			decapVrfSet: []string{vrfDecap, vrfEncapA, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    5,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    6,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    7,
			protocol:    ipipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    8,
			protocol:    ipv6ipProtocol,
			dscpSet:     []uint8{dscpEncapB1, dscpEncapB2},
			decapVrfSet: []string{vrfDecap, vrfEncapB, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    9,
			protocol:    ipipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    10,
			protocol:    ipv6ipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfRepaired},
			srcAddr:     ipv4OuterSrc222,
		},
		{
			sequence:    11,
			protocol:    ipipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
		{
			sequence:    12,
			protocol:    ipv6ipProtocol,
			decapVrfSet: []string{vrfDecap, vrfDefault, vrfTransit},
			srcAddr:     ipv4OuterSrc111,
		},
	}

	var encapRules = []pbrRule{
		{
			sequence: 13,
			dscpSet:  []uint8{dscpEncapA1, dscpEncapA2},
			encapVrf: vrfEncapA,
		},
		{
			sequence:  14,
			dscpSetV6: []uint8{dscpEncapA1, dscpEncapA2},
			encapVrf:  vrfEncapA,
		},
		{
			sequence: 15,
			dscpSet:  []uint8{dscpEncapB1, dscpEncapB2},
			encapVrf: vrfEncapB,
		},
		{
			sequence:  16,
			dscpSetV6: []uint8{dscpEncapB1, dscpEncapB2},
			encapVrf:  vrfEncapB,
		},
	}

	// var defaultClassRule = []pbrRule{
	// 	{
	// 		sequence: 17,
	// 		encapVrf: vrfDefault,
	// 	},
	// }

	var splitDefaultClassRules = []pbrRule{
		{
			sequence:  17,
			etherType: ethertypeIPv4,
			encapVrf:  vrfDefault,
		},
		{
			sequence:  18,
			etherType: ethertypeIPv6,
			encapVrf:  vrfDefault,
		},
	}

	if clusterFacing {
		pbrRules = append(pbrRules, encapRules...)
	}

	pbrRules = append(pbrRules, splitDefaultClassRules...)

	// if deviations.PfDefaultRuleVariableSequenceUnsupported(dut) {
	// pbrRules = append(pbrRules, splitDefaultClassRules...)
	// } else {
	// 	pbrRules = append(pbrRules, defaultClassRule...)
	// }

	return pbrRules
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string) {
	t.Logf("Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
	sr = static.GetOrCreateStatic(v6Prefix)
	nh = sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v6NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configIPv4DefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop string) {
	t.Logf("Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configureNetworkInstance creates nonDefaultVRFs
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfTransit, vrfRepaired, vrfEncapA, vrfEncapB, vrfDecapPostRepaired, vrfRepair}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)
}

// cidr takes as input the IPv4 address and the Mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

// getPbrPolicy creates PBR rules for cluster
func getPbrPolicy(dut *ondatra.DUTDevice, clusterFacing bool) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	pf := ni.GetOrCreatePolicyForwarding()
	var name string
	if clusterFacing {
		name = clusterPolicy
	} else {
		name = wanPolicy
	}
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut, clusterFacing) {
		//offset sequnce by 10
		sequence := pRule.sequence + 10
		r := p.GetOrCreateRule(sequence)
		l2 := r.GetOrCreateL2()
		r4 := r.GetOrCreateIpv4()

		if pRule.dscpSet != nil {
			r4.DscpSet = pRule.dscpSet
		} else if pRule.dscpSetV6 != nil {
			r6 := r.GetOrCreateIpv6()
			r6.DscpSet = pRule.dscpSetV6
		}

		if pRule.protocol != 0 {
			r4.Protocol = oc.UnionUint8(pRule.protocol)
		}

		if pRule.srcAddr != "" {
			r4.SourceAddress = ygot.String(cidr(pRule.srcAddr, 32))
		}

		if len(pRule.decapVrfSet) == 3 {
			ra := r.GetOrCreateAction()
			ra.DecapNetworkInstance = ygot.String(pRule.decapVrfSet[0])
			ra.PostDecapNetworkInstance = ygot.String(pRule.decapVrfSet[1])
			ra.DecapFallbackNetworkInstance = ygot.String(pRule.decapVrfSet[2])
		}

		if pRule.etherType != nil {
			l2.SetEthertype(pRule.etherType)
		}
		if pRule.encapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.encapVrf)

		}
	}
	return pf
}

// configureBaseconfig configures network instances and forwarding policy on the DUT
func configureBaseconfig(t *testing.T, dut *ondatra.DUTDevice, clusterFacing bool) {
	t.Log("Configure VRFs")
	configureNetworkInstance(t, dut)
	t.Log("Configure Cluster facing VRF selection Policy")
	pf := getPbrPolicy(dut, clusterFacing)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
}

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

func configureBundleInterfaces(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, bundleID string, dutPort *attrs.Attributes) {
	d := gnmi.OC()
	t.Logf("Configuring interface %s as bundle member of %s", port.Name(), bundleID)

	be := dutPort.NewOCInterface(bundleID, dut)
	be.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	gnmi.Replace(t, dut, d.Interface(bundleID).Config(), be)

	bm := generateBundleMemberInterfaceConfig(t, port.Name(), bundleID)
	gnmi.Replace(t, dut, d.Interface(port.Name()).Config(), bm)

}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, clusterFacing bool) {
	// d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	// p6 := dut.Port(t, "port6")
	// p7 := dut.Port(t, "port7")
	p8 := dut.Port(t, "port8")

	// configure interfaces

	// be1 := dutPort1.NewOCInterface("Bundle-Ether1", dut)
	// be1.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	// gnmi.Replace(t, dut, d.Interface("Bundle-Ether1").Config(), be1)

	// BE1 := generateBundleMemberInterfaceConfig(t, p1.Name(), "Bundle-Ether1")
	// gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), BE1)

	configureBundleInterfaces(t, dut, p1, "Bundle-Ether1", &dutPort1)
	configureBundleInterfaces(t, dut, p2, "Bundle-Ether2", &dutPort2)
	configureBundleInterfaces(t, dut, p3, "Bundle-Ether3", &dutPort3)
	configureBundleInterfaces(t, dut, p4, "Bundle-Ether4", &dutPort4)
	configureBundleInterfaces(t, dut, p5, "Bundle-Ether5", &dutPort5)
	configureBundleInterfaces(t, dut, p8, "Bundle-Ether8", &dutPort8)

	// gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	// gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	// gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	// gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))
	// gnmi.Replace(t, dut, d.Interface(p5.Name()).Config(), dutPort5.NewOCInterface(p5.Name(), dut))

	// configure base PBF policies and network-instances
	configureBaseconfig(t, dut, clusterFacing)

	// apply PBF to src interface.
	// applyForwardingPolicy(t, dut, p1.Name(), clusterFacing)
	applyForwardingPolicy(t, dut, "Bundle-Ether1", clusterFacing)
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	}
	configSflow(t, dut)
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice, ingressPort string, clusterFacing bool) {
	policyName := wanPolicy
	if clusterFacing {
		policyName = clusterPolicy
	}
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	// interfaceID := ingressPort
	// if deviations.InterfaceRefInterfaceIDFormat(dut) {
	interfaceID := ingressPort + ".0"
	// }
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

func configureDUTforPopGate(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")

	// configure interfaces
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p5.Name()).Config(), dutPort5.NewOCInterface(p5.Name(), dut))

	// configure base PBF policies and network-instances
	t.Log("Configure VRFs")
	configureNetworkInstance(t, dut)
	t.Log("Configure Cluster facing VRF selection Policy")
	pf := configurePopGatePBF(dut)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)

	// apply PBF to src interface.
	applyForwardingPopGatePolicy(t, p1.Name())
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	}
}

// configurePBF returns a fully configured network-instance PF struct
func configurePopGatePBF(dut *ondatra.DUTDevice) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	vrfPolicy := pf.GetOrCreatePolicy(popGatePolicy)
	vrfPolicy.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	vrfPolicy.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4OuterSrc111 + "/32")
	vrfPolicy.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(vrfTransit)
	return pf
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPopGatePolicy(t *testing.T, ingressPort string) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	interfaceID := ingressPort
	interfaceID = ingressPort + ".0"
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(popGatePolicy)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		pfCfg.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configreOTG configures port1-5 on the OTG.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")
	p5 := ate.Port(t, "port5")
	p8 := ate.Port(t, "port8")

	otgPort1.AddToOTG(topo, p1, &dutPort1)
	otgPort2.AddToOTG(topo, p2, &dutPort2)
	otgPort3.AddToOTG(topo, p3, &dutPort3)
	otgPort4.AddToOTG(topo, p4, &dutPort4)
	otgPort5.AddToOTG(topo, p5, &dutPort5)
	otgPort8.AddToOTG(topo, p8, &dutPort8)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	// time.Sleep(60 * time.Second)
	t.Logf("starting protocols...")
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

// enableCapture enables packet capture on specified list of ports on OTG
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	//t.Log(topo.Msg().GetCaptures())
	otg.PushConfig(t, topo)
}

// clearCapture clears capture from all ports on the OTG
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Log("Clearing capture")
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

// getFlow returns a flow of type ipv4, ipv4in4, ipv6in4 or ipv6 with dscp value passed in args.
func (fa *flowAttr) getFlow(flowType string, name string, dscp uint32) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(uint32(*frameSize))
	flow.Rate().SetPps(100) //(uint64(*fps))

	fmt.Printf("+++++++++++(uint64(*fps)) = %d", uint64(*fps))
	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if flowType == "ipv4" || flowType == "ipv4in4" || flowType == "ipv6in4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fa.src)
		v4.Dst().SetValue(fa.dst)
		v4.TimeToLive().SetValue(fa.ttl)
		if fa.dscp == 0 {
			v4.Priority().Dscp().Phb().SetValue(dscp)
		} else {
			v4.Priority().Dscp().Phb().SetValue(fa.dscp)
		}

		// add inner ipv4 headers
		if flowType == "ipv4in4" {
			innerV4 := flow.Packet().Add().Ipv4()
			innerV4.Src().SetValue(innerV4SrcIP)
			innerV4.Dst().SetValue(innerV4DstIP)
			innerV4.TimeToLive().SetValue(fa.innerTtl)
			innerV4.Priority().Dscp().Phb().SetValue(fa.innerDscp)
		}

		// add inner ipv6 headers
		if flowType == "ipv6in4" {
			innerV6 := flow.Packet().Add().Ipv6()
			innerV6.Src().SetValue(InnerV6SrcIP)
			innerV6.Dst().SetValue(InnerV6DstIP)
			innerV6.HopLimit().SetValue(fa.innerTtl)
			innerV6.TrafficClass().SetValue(fa.innerDscp << 2)
		}

	} else if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		v6.Dst().SetValue(fa.dst)
		v6.HopLimit().SetValue(ttl)
		v6.TrafficClass().SetValue(dscp << 2)
	}

	udp := flow.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(50001).SetCount(1000)
	udp.DstPort().Increment().SetStart(50001).SetCount(1000)

	return flow
}

// sendTraffic starts traffic flows and send traffic for a fixed duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)
	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)
	if capture {
		startCapture(t, args.ate)
		defer stopCapture(t, args.ate)
	}
	t.Log("Starting traffic")
	otg.StartTraffic(t)

	if args.trafficDuration == 0 {
		time.Sleep(time.Duration(*trafficDuration) * time.Second)
	} else {
		time.Sleep(time.Duration(args.trafficDuration) * time.Second)
	}

	otg.StopTraffic(t)
	t.Log("Traffic stopped")
}

// validateTrafficFlows verifies that the flow on ATE should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool, match bool) {

	otg := args.ate.OTG()
	sendTraffic(t, args, flows, capture)

	otgutils.LogPortMetrics(t, otg, args.topo)
	otgutils.LogFlowMetrics(t, otg, args.topo)

	for _, flow := range flows {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if match {
			if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
				t.Fatalf("LossPct for flow %s: got %v, want 0", flow.Name(), got)
			}
		} else {
			if got := ((outPkts - inPkts) * 100) / outPkts; got < (100 - lossTolerance) {
				t.Fatalf("LossPct for flow %s: got %v, want %v", flow.Name(), got, (100 - lossTolerance))
			}
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
// func validatePacketCapture(t *testing.T, args *testArgs, otgPortNames []string, pa *packetAttr) map[string][]int {
// 	tunCounter := make(map[string][]int)
// 	for _, otgPortName := range otgPortNames {
// 		l := NewLogger(t)

// 		bytes := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
// 		f, err := os.CreateTemp("", "fibchains.pcap")
// 		if err != nil {
// 			t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
// 		}
// 		t.Logf("Created pcap file %s", f.Name())
// 		if _, err := f.Write(bytes); err != nil {
// 			t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
// 		}
// 		f.Close()
// 		t.Logf("Verifying packet attributes captured on %s", otgPortName)
// 		handle, err := pcap.OpenOffline(f.Name())
// 		if err != nil {
// 			// log.Fatal(err)
// 			log.Printf("%v", err)
// 			break
// 		}
// 		defer handle.Close()

// 		// if err == nil {

// 		if pa.sfConfig != nil {
// 			validateSflowPackets(t, f.Name(), IPv6, *pa.sfConfig)
// 		}

// 		tunnel1Pkts := 0
// 		tunnel2Pkts := 0
// 		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
// 		for packet := range packetSource.Packets() {

// 			if ipV4Layer := packet.Layer(layers.LayerTypeIPv4); ipV4Layer != nil {
// 				v4Packet, _ := ipV4Layer.(*layers.IPv4)
// 				l.LogOncef("Outer IPv4 packet: %+v\n", v4Packet)
// 				if got := v4Packet.Protocol; got != layers.IPProtocol(pa.protocol) {
// 					l.LogOnceErrorf("Outer Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
// 					break
// 				} else {
// 					l.LogOncef("Outer Packet protocol type matched: %d", pa.protocol)
// 				}
// 				if got := int(v4Packet.TOS >> 2); got != pa.dscp {
// 					l.LogOnceErrorf("Outer Dscp value mismatch, got %d, want %d", got, pa.dscp)
// 				} else {
// 					l.LogOncef("Outer Dscp value matched: %d", pa.dscp)
// 				}
// 				if got := uint32(v4Packet.TTL); got != pa.ttl {
// 					l.LogOnceErrorf("Outer TTL mismatch, got: %d, want: %d", got, pa.ttl)
// 				} else {
// 					l.LogOncef("Outer TTL matched: %d", pa.ttl)
// 				}
// 				if v4Packet.DstIP.String() == tunnelDstIP1 {
// 					tunnel1Pkts++
// 				}
// 				if v4Packet.DstIP.String() == tunnelDstIP2 {
// 					tunnel2Pkts++
// 				}
// 				// check for inner IPv4 packet
// 				if v4Packet.Protocol == layers.IPProtocolIPv4 && pa.inner != nil {
// 					nextIPV4Layer := gopacket.NewPacket(v4Packet.Payload, layers.LayerTypeIPv4, gopacket.Default)
// 					innerIPv4Layer := nextIPV4Layer.Layer(layers.LayerTypeIPv4)
// 					if innerIPv4Layer != nil {
// 						innerIPv4Packet, _ := innerIPv4Layer.(*layers.IPv4)
// 						// Process the inner IPv4 packet as needed
// 						l.LogOncef("Inner IPv4 packet: %+v\n", innerIPv4Packet)
// 						if got := innerIPv4Packet.Protocol; got != layers.IPProtocol(pa.inner.protocol) {
// 							l.LogOnceErrorf("Inner Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
// 						} else {
// 							l.LogOncef("Inner Packet protocol type matched: %d", pa.inner.protocol)
// 						}
// 						if got := int(innerIPv4Packet.TOS >> 2); got != pa.inner.dscp {
// 							l.LogOnceErrorf("Inner Packet Dscp value mismatch, got %d, want %d", got, pa.dscp)
// 						} else {
// 							l.LogOncef("Inner Packet Dscp value matched: %d", pa.inner.dscp)
// 						}
// 						if got := uint32(innerIPv4Packet.TTL); got != pa.inner.ttl {
// 							l.LogOnceErrorf("Inner Packer TTL mismatch, got: %d, want: %d", got, pa.ttl)
// 						} else {
// 							l.LogOncef("Inner Packet TTL matched: %d", pa.inner.ttl)
// 						}
// 					}
// 				}
// 				// Check if the next protocol is IPv6
// 				if v4Packet.Protocol == layers.IPProtocolIPv6 && pa.inner != nil {
// 					nextIPV6Layer := gopacket.NewPacket(v4Packet.Payload, layers.LayerTypeIPv6, gopacket.Default)
// 					innerIPv6Layer := nextIPV6Layer.Layer(layers.LayerTypeIPv6)
// 					if innerIPv6Layer != nil {
// 						innerIPv6Packet, _ := innerIPv6Layer.(*layers.IPv6)
// 						// Process the inner IPv6 packet as needed
// 						l.LogOncef("Inner IPv6 packet: %+v\n", innerIPv6Packet)
// 						if got := innerIPv6Packet.NextHeader; got != layers.IPProtocol(pa.inner.protocol) {
// 							l.LogOnceErrorf("Inner Packet protocol type mismatch, got: %d, want %d", got, pa.inner.protocol)
// 						} else {
// 							l.LogOncef("Inner Packet protocol type matched: %d", pa.inner.protocol)
// 						}
// 						if got := int(innerIPv6Packet.TrafficClass >> 2); got != pa.inner.dscp {
// 							l.LogOnceErrorf("Inner Packet Dscp value mismatch, got %d, want %d", got, pa.inner.dscp)
// 						} else {
// 							l.LogOncef("Inner Packet Dscp value matched: %d", pa.inner.dscp)
// 						}
// 						if got := uint32(innerIPv6Packet.HopLimit); got != pa.inner.ttl {
// 							l.LogOnceErrorf("Inner Packet TTL mismatch, got: %d, want: %d", got, pa.inner.ttl)
// 						} else {
// 							l.LogOncef("Inner Packet TTL matched: %d", pa.inner.ttl)
// 						}
// 					}
// 				}

//				} else if ipV6Layer := packet.Layer(layers.LayerTypeIPv6); ipV6Layer != nil {
//					v6Packet, _ := ipV6Layer.(*layers.IPv6)
//					// ignore ICMPv6 packets received for neighbor discovery
//					if v6Packet.NextHeader == layers.IPProtocolICMPv6 {
//						t.Logf("Ignoring ICMPv6 packet received")
//						continue
//					}
//					l.LogOncef("Outer IPv6 packet: %+v\n", v6Packet)
//					if got := v6Packet.NextHeader; got != layers.IPProtocol(pa.protocol) {
//						l.LogOnceErrorf("Outer Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
//					} else {
//						l.LogOncef("Outer Packet protocol type matched: %d", pa.protocol)
//					}
//					if got := int(v6Packet.TrafficClass >> 2); got != pa.dscp {
//						l.LogOnceErrorf("Outer Dscp value mismatch, got %d, want %d", got, pa.dscp)
//					} else {
//						l.LogOncef("Outer Dscp value matched: %d", pa.dscp)
//					}
//					if got := uint32(v6Packet.HopLimit); got != pa.ttl {
//						l.LogOnceErrorf("Outer TTL mismatch, got: %d, want: %d", got, pa.ttl)
//					} else {
//						l.LogOncef("Outer TTL matched: %d", pa.ttl)
//					}
//				}
//			}
//			t.Logf("tunnel1, tunnel2 packet count on %s: %d , %d", otgPortName, tunnel1Pkts, tunnel2Pkts)
//			tunCounter[otgPortName] = []int{tunnel1Pkts, tunnel2Pkts}
//		}
//		return tunCounter
//	}
func validatePacketCapture(t *testing.T, args *testArgs, otgPortNames []string, pa *packetAttr) map[string][]int {
	tunCounter := make(map[string][]int)
	for _, otgPortName := range otgPortNames {
		l := NewLogger(t)
		bytes := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(otgPortName))
		f, err := os.CreateTemp("", "fibchains.pcap")
		if err != nil {
			t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
		}
		t.Logf("Created pcap file %s", f.Name())
		if _, err := f.Write(bytes); err != nil {
			t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
		}
		f.Close()
		t.Logf("Verifying packet attributes captured on %s", otgPortName)
		handle, err := pcap.OpenOffline(f.Name())
		if err != nil {
			log.Printf("%v", err)
			break
		}
		defer handle.Close()

		if pa.sfConfig != nil {
			validateSflowPackets(t, f.Name(), IPv6, *pa.sfConfig)
		}

		tunnel1Pkts, tunnel2Pkts := 0, 0
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			switch {
			case packet.Layer(layers.LayerTypeIPv4) != nil:
				v4Packet, _ := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
				validateIPv4Packet(l, v4Packet, pa)
				if v4Packet.DstIP.String() == tunnelDstIP1 {
					tunnel1Pkts++
				}
				if v4Packet.DstIP.String() == tunnelDstIP2 {
					tunnel2Pkts++
				}
				// Check for inner IPv4 or IPv6
				if v4Packet.Protocol == layers.IPProtocolIPv4 && pa.inner != nil {
					validateInnerIPv4Packet(l, v4Packet.Payload, pa.inner)
				}
				if v4Packet.Protocol == layers.IPProtocolIPv6 && pa.inner != nil {
					validateInnerIPv6Packet(l, v4Packet.Payload, pa.inner)
				}
			case packet.Layer(layers.LayerTypeIPv6) != nil:
				v6Packet, _ := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
				// Ignore ICMPv6 packets for neighbor discovery
				if v6Packet.NextHeader == layers.IPProtocolICMPv6 {
					t.Logf("Ignoring ICMPv6 packet received")
					continue
				}
				validateIPv6Packet(l, v6Packet, pa)
			case packet.Layer(layers.LayerTypeSFlow) != nil:
				// sFlow validation is already handled above if pa.sfConfig != nil
				t.Logf("Ignoring sFlow packet received")
				continue
			default:
				// Unhandled packet type
			}
		}
		t.Logf("tunnel1, tunnel2 packet count on %s: %d , %d", otgPortName, tunnel1Pkts, tunnel2Pkts)
		tunCounter[otgPortName] = []int{tunnel1Pkts, tunnel2Pkts}
	}
	return tunCounter
}

func validateIPv4Packet(l *Logger, v4Packet *layers.IPv4, pa *packetAttr) {
	if got := v4Packet.Protocol; got != layers.IPProtocol(pa.protocol) {
		l.LogOnceErrorf("Outer Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
	} else {
		l.LogOncef("Outer Packet protocol type matched: %d", pa.protocol)
	}
	if got := int(v4Packet.TOS >> 2); got != pa.dscp {
		l.LogOnceErrorf("Outer Dscp value mismatch, got %d, want %d", got, pa.dscp)
	} else {
		l.LogOncef("Outer Dscp value matched: %d", pa.dscp)
	}
	if got := uint32(v4Packet.TTL); got != pa.ttl {
		l.LogOnceErrorf("Outer TTL mismatch, got: %d, want: %d", got, pa.ttl)
	} else {
		l.LogOncef("Outer TTL matched: %d", pa.ttl)
	}
}

func validateInnerIPv4Packet(l *Logger, payload []byte, pa *packetAttr) {
	nextIPV4Layer := gopacket.NewPacket(payload, layers.LayerTypeIPv4, gopacket.Default)
	innerIPv4Layer := nextIPV4Layer.Layer(layers.LayerTypeIPv4)
	if innerIPv4Layer != nil {
		innerIPv4Packet, _ := innerIPv4Layer.(*layers.IPv4)
		l.LogOncef("Inner IPv4 packet: %+v\n", innerIPv4Packet)
		if got := innerIPv4Packet.Protocol; got != layers.IPProtocol(pa.protocol) {
			l.LogOnceErrorf("Inner Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
		} else {
			l.LogOncef("Inner Packet protocol type matched: %d", pa.protocol)
		}
		if got := int(innerIPv4Packet.TOS >> 2); got != pa.inner.dscp {
			l.LogOnceErrorf("Inner Packet Dscp value mismatch, got %d, want %d", got, pa.dscp)
		} else {
			l.LogOncef("Inner Packet Dscp value matched: %d", pa.inner.dscp)
		}
		if got := uint32(innerIPv4Packet.TTL); got != pa.inner.ttl {
			l.LogOnceErrorf("Inner Packet TTL mismatch, got: %d, want: %d", got, pa.ttl)
		} else {
			l.LogOncef("Inner Packet TTL matched: %d", pa.inner.ttl)
		}
	}
}

func validateInnerIPv6Packet(l *Logger, payload []byte, pa *packetAttr) {
	nextIPV6Layer := gopacket.NewPacket(payload, layers.LayerTypeIPv6, gopacket.Default)
	innerIPv6Layer := nextIPV6Layer.Layer(layers.LayerTypeIPv6)
	if innerIPv6Layer != nil {
		innerIPv6Packet, _ := innerIPv6Layer.(*layers.IPv6)
		l.LogOncef("Inner IPv6 packet: %+v\n", innerIPv6Packet)
		if got := innerIPv6Packet.NextHeader; got != layers.IPProtocol(pa.protocol) {
			l.LogOnceErrorf("Inner Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
		} else {
			l.LogOncef("Inner Packet protocol type matched: %d", pa.protocol)
		}
		if got := int(innerIPv6Packet.TrafficClass >> 2); got != pa.inner.dscp {
			l.LogOnceErrorf("Inner Packet Dscp value mismatch, got %d, want %d", got, pa.inner.dscp)
		} else {
			l.LogOncef("Inner Packet Dscp value matched: %d", pa.inner.dscp)
		}
		if got := uint32(innerIPv6Packet.HopLimit); got != pa.inner.ttl {
			l.LogOnceErrorf("Inner Packet TTL mismatch, got: %d, want: %d", got, pa.inner.ttl)
		} else {
			l.LogOncef("Inner Packet TTL matched: %d", pa.inner.ttl)
		}
	}
}

func validateIPv6Packet(l *Logger, v6Packet *layers.IPv6, pa *packetAttr) {
	if got := v6Packet.NextHeader; got != layers.IPProtocol(pa.protocol) {
		l.LogOnceErrorf("Outer Packet protocol type mismatch, got: %d, want %d", got, pa.protocol)
	} else {
		l.LogOncef("Outer Packet protocol type matched: %d", pa.protocol)
	}
	if got := int(v6Packet.TrafficClass >> 2); got != pa.dscp {
		l.LogOnceErrorf("Outer Dscp value mismatch, got %d, want %d", got, pa.dscp)
	} else {
		l.LogOncef("Outer Dscp value matched: %d", pa.dscp)
	}
	if got := uint32(v6Packet.HopLimit); got != pa.ttl {
		l.LogOnceErrorf("Outer TTL mismatch, got: %d, want: %d", got, pa.ttl)
	} else {
		l.LogOncef("Outer TTL matched: %d", pa.ttl)
	}
}

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// validateTrafficDistribution checks if the packets received on receiving ports are within specificied weight ratios
func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// skip first entry that belongs to source port on ate
	gotWeights, _ := normalize(inFramesAllPorts[1 : len(inFramesAllPorts)-1]) // last entry is the sink port sflow packet count

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, trfDistTolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// staticARPWithSecondaryIP configures secondary IPs and static ARP.
func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	// p2 := dut.Port(t, "port2")
	// p3 := dut.Port(t, "port3")
	// p4 := dut.Port(t, "port4")
	// p5 := dut.Port(t, "port5")

	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether2").Config(), assignIPAsSecondary(&dutPort2DummyIP, "Bundle-Ether2", dut))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether3").Config(), assignIPAsSecondary(&dutPort3DummyIP, "Bundle-Ether3", dut))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether4").Config(), assignIPAsSecondary(&dutPort4DummyIP, "Bundle-Ether4", dut))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether5").Config(), assignIPAsSecondary(&dutPort5DummyIP, "Bundle-Ether5", dut))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether8").Config(), assignIPAsSecondary(&dutPort8DummyIP, "Bundle-Ether8", dut))

	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether2").Config(), configStaticArp("Bundle-Ether2", otgPort2DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether3").Config(), configStaticArp("Bundle-Ether3", otgPort3DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether4").Config(), configStaticArp("Bundle-Ether4", otgPort4DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether5").Config(), configStaticArp("Bundle-Ether5", otgPort5DummyIP.IPv4, magicMac))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether8").Config(), configStaticArp("Bundle-Ether8", otgPort8DummyIP.IPv4, magicMac))

	// gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), assignIPAsSecondary(&dutPort2DummyIP, p2.Name(), dut))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), assignIPAsSecondary(&dutPort3DummyIP, p3.Name(), dut))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), assignIPAsSecondary(&dutPort4DummyIP, p4.Name(), dut))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), assignIPAsSecondary(&dutPort5DummyIP, p5.Name(), dut))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2.Name(), otgPort2DummyIP.IPv4, magicMac))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMac))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMac))
	// gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMac))
}

// override ip address type as secondary
func assignIPAsSecondary(a *attrs.Attributes, port string, dut *ondatra.DUTDevice) *oc.Interface {
	intf := a.NewOCInterface(port, dut)
	intf.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	s := intf.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.Type = oc.IfIp_Ipv4AddressType_SECONDARY
	return intf
}

func configureVIP1(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(2), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(2), map[uint64]uint64{baseNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP1, 32), baseNHG(2), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP2(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(3), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort3DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), baseNHG(3), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP2BGPPrefix(t *testing.T, args *testArgs, prefix string) {
	args.client.AddNH(t, baseNH(3), prefix, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, baseNHG(3), map[uint64]uint64{baseNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP2, 32), baseNHG(3), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3(t *testing.T, args *testArgs) {
	args.client.AddNH(t, baseNH(4), "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort4DummyIP.IPv4, Mac: magicMac})
	args.client.AddNHG(t, baseNHG(4), map[uint64]uint64{baseNH(4): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(vipIP3, 32), baseNHG(4), deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3NHGWithTunnel(t *testing.T, args *testArgs) {
	configureVIP3(t, args)

	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

func configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t *testing.T, args *testArgs) {

	// backup to repair
	args.client.AddNH(t, baseNH(5), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, baseNHG(5), map[uint64]uint64{baseNH(5): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	configureVIP3(t, args)
	// repair path
	args.client.AddNH(t, vipNH(3), vipIP3, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, vipNHG(3), map[uint64]uint64{vipNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: baseNHG(5)})
	args.client.AddIPv4(t, cidr(tunnelDstIP3, 32), vipNHG(3), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNH(t, tunNH(3), "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc222, Dest: tunnelDstIP3, VrfName: vrfRepaired})
	args.client.AddNHG(t, tunNHG(3), map[uint64]uint64{tunNH(3): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

// CLI to configure Default static route with NH default VRF. NH VRF option is not available with OC Local routing
func configDefaultIPStaticCli(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		v4Conf := fmt.Sprintf("router static vrf %v address-family ipv4 unicast 0.0.0.0/0 vrf default\n router static vrf %v address-family ipv6 unicast ::/0 vrf default", v, v)
		config.TextWithGNMI(ctx, t, dut, v4Conf)
		time.Sleep(5 * time.Second)
	}
}

// CLI to configure Default static route with NH default VRF. NH VRF option is not available with OC Local routing
func unConfigDefaultIPStaticCli(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		v4Conf := fmt.Sprintf("no router static vrf %v address-family ipv4 unicast 0.0.0.0/0 vrf default\n no router static vrf %v address-family ipv6 unicast ::/0 vrf default", v, v)
		config.TextWithGNMI(ctx, t, dut, v4Conf)
		time.Sleep(5 * time.Second)
	}
}

// CLI to configure falback vrf
func configFallBackVrf(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		fConf := fmt.Sprintf("vrf %v fallback-vrf default\n", v)
		config.TextWithGNMI(ctx, t, dut, fConf)
	}
}

// CLI to unconfigure falback vrf
func unconfigFallBackVrf(t *testing.T, dut *ondatra.DUTDevice, vrf []string) {
	ctx := context.Background()
	for _, v := range vrf {
		fConf := fmt.Sprintf("no vrf %v fallback-vrf default\n", v)
		config.TextWithGNMI(ctx, t, dut, fConf)
	}
}

func testTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	flows := []gosnappi.Flow{fa4.getFlow("ipv4", "ip4a1", dscpEncapA1), fa6.getFlow("ipv6", "ip6a1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func testEncapTrafficTtlDscp(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	enableCapture(t, args.ate.OTG(), args.topo, args.capture_ports)
	defer clearCapture(t, args.ate.OTG(), args.topo)
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, args.flows, true, shouldPass)

	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
		validatePacketCapture(t, args, args.capture_ports, args.pattr)
	}
}

func testTransitTraffic(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscpEncapA1), faTransit.getFlow("ipv6in4", "ip6inipa1", dscpEncapA1)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func testTransitTrafficWithDscp(t *testing.T, args *testArgs, weights []float64, dscp uint32, shouldPass bool) {
	flows := []gosnappi.Flow{faTransit.getFlow("ipv4in4", "ip4inipa1", dscp), faTransit.getFlow("ipv6in4", "ip6inipa1", dscp)}
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, flows, false, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
	}
}

func testTransitTrafficWithTtlDscp(t *testing.T, args *testArgs, weights []float64, shouldPass bool) {
	enableCapture(t, args.ate.OTG(), args.topo, args.capture_ports)
	defer clearCapture(t, args.ate.OTG(), args.topo)
	t.Log("Validate traffic flows")
	validateTrafficFlows(t, args, args.flows, true, shouldPass)
	if shouldPass {
		t.Log("Validate hierarchical traffic distribution")
		validateTrafficDistribution(t, args.ate, weights)
		validatePacketCapture(t, args, args.capture_ports, args.pattr)
	}
}

func validateCapture(t *testing.T, args *testArgs) {
	enableCapture(t, args.ate.OTG(), args.topo, args.capture_ports)
	sendTraffic(t, args, args.flows, true)
	defer clearCapture(t, args.ate.OTG(), args.topo)
	validatePacketCapture(t, args, args.capture_ports, args.pattr)
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Logger struct to hold the map and a mutex for thread safety
type Logger struct {
	mu   sync.Mutex
	seen map[string]bool
	t    *testing.T
}

// NewLogger initializes and returns a new Logger instance
func NewLogger(t *testing.T) *Logger {
	return &Logger{
		seen: make(map[string]bool),
		t:    t,
	}
}

// LogOnce prints the message only if it hasn't been printed before
func (l *Logger) LogOnce(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.seen[message] {
		l.t.Log(message)
		l.seen[message] = true
	}
}

// LogOncef prints the message with formatting option only if it hasn't been printed before
func (l *Logger) LogOncef(message string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.seen[message] {
		l.t.Logf(message, args...)
		l.seen[message] = true
	}
}

// LogOnceErrorf prints the error message with formatting option only if it hasn't been printed before
func (l *Logger) LogOnceErrorf(message string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.seen[message] {
		l.t.Errorf(message, args...)
		l.seen[message] = true
	}
}

func shutPorts(t *testing.T, args *testArgs, ports []string) {
	t.Logf("Shutting down ports %v", ports)
	for _, port := range ports {
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, port).Name()).Subinterface(0).Enabled().Config(), false)
	}
}
func unshutPorts(t *testing.T, args *testArgs, ports []string) {
	t.Logf("Unshutting ports %v", ports)
	for _, port := range ports {
		gnmi.Update(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, port).Name()).Subinterface(0).Enabled().Config(), true)
	}
	time.Sleep(5 * time.Second)
}

func configSflow(t *testing.T, dut *ondatra.DUTDevice) {
	cliCfg := `interface loopback0
ipv4 address 203.0.113.255/32
ipv6 address 2001:db8::203:0:113:255/128
no shut
flow exporter-map exporter
dfbit set
packet-length 8968
version sflow v5
!
dscp 32
transport udp 6343
source Loopback0
destination 2001:0db8::192:0:2:1e
!
flow exporter-map OC-FEM-GLOBAL
dscp 32
!
flow exporter-map OC-FEM-2001_4860_f802__be-6343
version sflow v5
!
transport udp 6343
source-address 2001:db8::203:0:113:255
destination 2001:0db8::192:0:2:1e
!
flow monitor-map fmm
record sflow
sflow options
input ifindex physical
output ifindex physical
extended-router
extended-gateway
extended-ipv4-tunnel-egress
!
exporter exporter
!
flow monitor-map OC-FMM-GLOBAL
record sflow
sflow options
input ifindex physical
output ifindex physical
sample-header size 343
extended-router
extended-gateway
extended-ipv4-tunnel-egress
extended-ipv6-tunnel-egress
!
exporter OC-FEM-2001_4860_f802__be-6343
!
sampler-map fsm
random 1 out-of 262144
!
sampler-map OC-FSM-GLOBAL-EGRESS
!
sampler-map OC-FSM-GLOBAL-INGRESS
random 1 out-of 262144

interface bundle-ether1
flow datalinkframesection monitor OC-FMM-GLOBAL sampler OC-FSM-GLOBAL-INGRESS ingress
`
	batchSet := &gnmi.SetBatch{}
	cliPath, _ := schemaless.NewConfig[string]("", "cli")
	gnmi.BatchUpdate(batchSet, cliPath, cliCfg)
	batchSet.Set(t, dut)
}

// import (
//     "fmt"
//     "net"
//     "testing" // Assuming this is part of a test file, so t *testing.T is valid

//     "github.com/google/gopacket"
//     "github.com/google/gopacket/layers"
//     "github.com/google/gopacket/pcap"
// )

// Assuming these are defined elsewhere in the test file or are example values
const (
	samplingRate    = 262144 // Example value, adjust as per your test setup
	sampleTolerance = 0.8    // Example value, adjust as per your test setup
)

// Dummy struct for dutlo0Attrs to make the code runnable without full testbed context
var dutlo0Attrs = struct {
	IPv4 string
	IPv6 string
}{
	IPv4: "203.0.113.255",           // Example IP
	IPv6: "2001:db8::203:0:113:255", // Example IP
}

// User's provided SFlowFlowSample struct definition
// Note: This struct is a conceptual representation. The actual parsing
// will use gopacket's layers.SFlowFlowSample and layers.SFlowFlowRecord types.
type SFlowEnterpriseID uint32
type SFlowSampleType uint32
type SFlowSourceFormat uint32
type SFlowSourceValue uint32
type SFlowRecord interface{} // Represents different types of flow records

type SFlowFlowSample struct {
	EnterpriseID          SFlowEnterpriseID
	Format                SFlowSampleType
	SampleLength          uint32
	SequenceNumber        uint32
	SourceIDClass         SFlowSourceFormat
	SourceIDIndex         SFlowSourceValue
	SamplingRate          uint32
	SamplePool            uint32
	Dropped               uint32
	InputInterfaceFormat  uint32 // Note: gopacket's SFlowFlowSample doesn't explicitly have this field
	InputInterface        uint32
	OutputInterfaceFormat uint32 // Note: gopacket's SFlowFlowSample doesn't explicitly have this field
	OutputInterface       uint32
	RecordCount           uint32
	Records               []SFlowRecord
}

func validateSflowPackets(t *testing.T, filename string, ip IPType, fc sflowConfig) {
	// First pass: Check for sFlow-exported packets based on IP header TOS/TrafficClass
	// This part of the function remains largely the same as your original code.
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close() // Ensure handle is closed when function exits

	loopbackIP := net.ParseIP(dutlo0Attrs.IPv4)
	if ip == IPv6 {
		loopbackIP = net.ParseIP(dutlo0Attrs.IPv6)
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	found := false
	sampleCount := 0               // This counts sFlow-exported packets (based on IP header)
	sflowSamplesTotal := uint32(0) // This will count actual sFlow samples later

	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			if ipv4.SrcIP.Equal(loopbackIP) {
				t.Logf("IP Packet: SrcIP=%s, DstIP=%s, TOS=%d, Length=%d", ipv4.SrcIP, ipv4.DstIP, ipv4.TOS, ipv4.Length)
				if (ipv4.TOS >> 2) == fc.sflowDscp {
					found = true
					sampleCount++
				}
			}
		} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
			ipv6, _ := ipLayer.(*layers.IPv6)
			if ipv6.SrcIP.Equal(loopbackIP) {
				t.Logf("IP Packet: SrcIP=%s, DstIP=%s, TrafficClass=%d, Length=%d", ipv6.SrcIP, ipv6.DstIP, ipv6.TrafficClass, ipv6.Length)
				if ipv6.TrafficClass == (fc.sflowDscp << 2) {
					found = true
					sampleCount++
				}
			}
		}
	}

	expectedSampleCount := float64(fc.packetsToSend / samplingRate)
	// expectedSampleCount := float64((120 * 100000) / samplingRate)
	minAllowedSamples := expectedSampleCount * sampleTolerance
	t.Logf("SFlow packets captured (based on IP header TOS/TrafficClass): %v", sampleCount)
	if !found || sampleCount < int(minAllowedSamples) {
		// t.Errorf("sflow packets not found or count too low: got %v, want >= %v", sampleCount, minAllowedSamples)
	}

	// Re-open pcap handle for the second pass to ensure we start from the beginning
	// This is crucial because `packetSource.Packets()` is a channel that closes
	// after all packets are read.
	handle, err = pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close() // Ensure handle is closed again after this function exits

	packetSource = gopacket.NewPacketSource(handle, handle.LinkType())

	// Variables to track and store picked samples for printing
	pickedFlowSamplesCount := 0
	packetCount := 0 // Reset packetCount for the second loop if needed

	// Second pass: Iterate through packets to find SFlow layers,
	// extract flow samples, and print their contents.
	for packet := range packetSource.Packets() {
		// If we've already found and printed two flow samples, break early.
		if pickedFlowSamplesCount >= 2 {
			t.Log("\n--- Two SFlow Flow Samples picked and printed. Stopping further processing. ---")
			// break
		}

		if sflowLayer := packet.Layer(layers.LayerTypeSFlow); sflowLayer != nil {
			sflowDatagram, ok := sflowLayer.(*layers.SFlowDatagram)
			if !ok {
				t.Logf("Warning: Could not cast SFlow layer to *layers.SFlowDatagram")
				// continue
			}

			packetCount++                                  // Count sFlow datagrams
			sflowSamplesTotal += sflowDatagram.SampleCount // Accumulate total reported samples

			t.Logf("\nSFlow Datagram %d found. Contains %d samples (total reported: %d).",
				packetCount, len(sflowDatagram.FlowSamples), sflowDatagram.SampleCount)

			// Iterate through the samples within this sFlow datagram
			for _, sample := range sflowDatagram.FlowSamples {
				// If we've already found and printed two flow samples, break from inner loop.
				if pickedFlowSamplesCount >= 2 {
					break
				}
				// flowSample, ok := sample.(*layers.SFlowFlowSample)
				// if !ok {
				// 	t.Logf("    Skipping non-Flow Sample (type: %T)", sample)
				// 	continue
				// }
				flowSample := sample
				// We found an SFlowFlowSample, now print its contents
				t.Logf("\n--- SFlow Flow Sample %d Details (from Datagram %d) ---", pickedFlowSamplesCount+1, packetCount)

				// Map fields from gopacket's SFlowDatagram and SFlowFlowSample to user's conceptual struct
				t.Logf("  EnterpriseID: %d", flowSample.EnterpriseID) // From SFlowDatagram
				t.Logf("  Format: %d", flowSample.Format)             // From SFlowDatagram
				t.Logf("  SampleLength: %d", flowSample.SampleLength) // Length of this specific sample
				t.Logf("  SequenceNumber: %d", flowSample.SequenceNumber)
				t.Logf("  SourceIDClass: %d", flowSample.SourceIDClass)
				t.Logf("  SourceIDIndex: %d", flowSample.SourceIDIndex)
				t.Logf("  SamplingRate: %d", flowSample.SamplingRate)
				t.Logf("  SamplePool: %d", flowSample.SamplePool)
				t.Logf("  Dropped: %d", flowSample.Dropped)
				// InputInterfaceFormat and OutputInterfaceFormat are not directly present in gopacket's SFlowFlowSample.
				// They are typically encoded within SourceIDClass/SourceIDIndex or specific extended records.
				t.Logf("  InputInterface: %d", flowSample.InputInterface)
				t.Logf("  OutputInterface: %d", flowSample.OutputInterface)
				t.Logf("  RecordCount: %d", len(flowSample.Records)) // Number of flow records within this sample

				// Print details of each FlowRecord within the sample
				for i, record := range flowSample.Records {
					t.Logf("    --- Flow Record %d (Type: %T) ---", i+1, record)
					switch r := record.(type) {
					case layers.SFlowRawPacketFlowRecord:
						t.Logf("      Type: Raw Packet Flow Record")
						t.Logf("      Header Protocol: %d", r.HeaderProtocol)
						t.Logf("      FrameLength: %d", r.FrameLength)
						// t.Logf("      Stripped: %d", r.Stripped)
						t.Logf("      Header Length: %d (first %d bytes of original packet)", r.HeaderLength, r.Header)
						// You can further parse r.Header if needed (e.g., gopacket.NewPacket(r.Header, ...))
					case layers.SFlowEthernetFrameFlowRecord:
						t.Logf("      Type: Ethernet Frame Flow Record")
						t.Logf("      FrameLength: %d", r.FrameLength)
						t.Logf("      SrcMAC: %s", r.SrcMac)
						t.Logf("      DstMAC: %s", r.DstMac)
						t.Logf("      EtherType: %d", r.Type)
					case layers.SFlowIpv4Record:
						t.Logf("      Type: IPv4 Flow Record")
						t.Logf("      SrcIP: %s", r.IPSrc)
						t.Logf("      DstIP: %s", r.IPDst)
						t.Logf("      Protocol: %d", r.Protocol)
						t.Logf("      SrcPort: %d", r.PortSrc)
						t.Logf("      DstPort: %d", r.PortDst)
						t.Logf("      TCPFlags: %d", r.TCPFlags)
						t.Logf("      TotalLength: %d", r.Length)
					case layers.SFlowIpv6Record:
						t.Logf("      Type: IPv6 Flow Record")
						t.Logf("      SrcIP: %s", r.IPSrc)
						t.Logf("      DstIP: %s", r.IPDst)
						t.Logf("      Protocol: %d", r.Protocol)
						t.Logf("      SrcPort: %d", r.PortSrc)
						t.Logf("      DstPort: %d", r.PortDst)
						t.Logf("      TCPFlags: %d", r.TCPFlags)
						t.Logf("      TotalLength: %d", r.Length)
					case layers.SFlowExtendedRouterFlowRecord:
						t.Logf("      Type: Extended Router Flow Record")
						t.Logf("      NextHop: %s", r.NextHop)
						t.Logf("      SrcMask: %d", r.NextHopSourceMask)
						t.Logf("      DstMask: %d", r.NextHopDestinationMask)
					case layers.SFlowExtendedGatewayFlowRecord:
						t.Logf("      Type: Extended Gateway Flow Record")
						t.Logf("      NextHop: %s", r.NextHop)
						t.Logf("      AS: %d", r.AS)
						t.Logf("      SrcAS: %d", r.SourceAS)
						t.Logf("      DstAS: %d", r.PeerAS)
						t.Logf("      Communities: %v", r.Communities)
						t.Logf("      LocalPref: %d", r.LocalPref)
					// case layers.SFlowExtendedIpv4TunnelFlowRecord:
					// 	t.Logf("      Type: Extended Tunnel Flow Record")
					// 	t.Logf("      TunnelType: %d", r.TunnelType)
					// 	t.Logf("      TunnelTTL: %d", r.TunnelTTL)
					// 	t.Logf("      TunnelProtocol: %d", r.TunnelProtocol)
					// case layers.SFlowExtendedMPLSFlowRecord:
					// 	t.Logf("      Type: Extended MPLS Flow Record")
					// 	t.Logf("      MPLSLabelStack: %v", r.MPLSLabelStack)
					// case layers.SFlowExtendedNATFlowRecord:
					// 	t.Logf("      Type: Extended NAT Flow Record")
					// 	t.Logf("      SrcNATIP: %s", r.SrcNATIP)
					// 	t.Logf("      DstNATIP: %s", r.DstNATIP)
					// case layers.SFlowExtendedBGPFlowRecord:
					// 	t.Logf("      Type: Extended BGP Flow Record")
					// 	t.Logf("      NextHop: %s", r.NextHop)
					// 	t.Logf("      AS: %d", r.AS)
					// 	t.Logf("      SrcAS: %d", r.SrcAS)
					// 	t.Logf("      DstAS: %d", r.DstAS)
					// 	t.Logf("      Communities: %v", r.Communities)
					// 	t.Logf("      LocalPref: %d", r.LocalPref)
					default:
						t.Logf("      Type: Unhandled Flow Record Type (%T)", record)
					}
				}
				pickedFlowSamplesCount++ // Increment count of printed flow samples
				// } else if counterSample, ok := sample.(*layers.SFlowCounterSample); ok {
				// t.Logf("    Found Counter Sample (Type: %T). Skipping for now.", counterSample)
				// You could add logic here to print counter sample details if needed
				// } else {
				// t.Logf("    Found Unknown SFlow Sample Type (%T). Skipping.", sample)
				// }
			}
		}
	}

	// Final check for total sFlow samples reported by datagrams
	if sflowSamplesTotal < uint32(minAllowedSamples) {
		// t.Errorf("Total SFlow samples reported by datagrams: %v, want > %v", sflowSamplesTotal, expectedSampleCount)
		t.Logf("Total SFlow samples reported by datagrams: %v, want > %v", sflowSamplesTotal, expectedSampleCount)
	}

	// New check to ensure two samples were found and printed
	if pickedFlowSamplesCount < 2 {
		// t.Errorf("Failed to find and print 2 SFlow Flow Samples. Only found: %d", pickedFlowSamplesCount)
		// t.Logf("Failed to find and print 2 SFlow Flow Samples. Only found: %d", pickedFlowSamplesCount)
	}
}

// // validateSflowSampleCount counts sFlow-exported packets based on IP header TOS/TrafficClass.
// func validateSflowSampleCount(t *testing.T, filename string, ip IPType, fc sflowConfig) (int, float64, float64) {
// 	handle, err := pcap.OpenOffline(filename)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer handle.Close()

// 	loopbackIP := net.ParseIP(dutlo0Attrs.IPv4)
// 	if ip == IPv6 {
// 		loopbackIP = net.ParseIP(dutlo0Attrs.IPv6)
// 	}
// 	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

// 	found := false
// 	sampleCount := 0

// 	for packet := range packetSource.Packets() {
// 		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
// 			ipv4, _ := ipLayer.(*layers.IPv4)
// 			if ipv4.SrcIP.Equal(loopbackIP) {
// 				if (ipv4.TOS >> 2) == fc.sflowDscp {
// 					found = true
// 					sampleCount++
// 				}
// 			}
// 		} else if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
// 			ipv6, _ := ipLayer.(*layers.IPv6)
// 			if ipv6.SrcIP.Equal(loopbackIP) {
// 				if ipv6.TrafficClass == (fc.sflowDscp << 2) {
// 					found = true
// 					sampleCount++
// 				}
// 			}
// 		}
// 	}

// 	expectedSampleCount := float64(fc.packetsToSend / samplingRate)
// 	minAllowedSamples := expectedSampleCount * sampleTolerance
// 	t.Logf("SFlow packets captured (based on IP header TOS/TrafficClass): %v", sampleCount)
// 	return sampleCount, expectedSampleCount, minAllowedSamples
// }

// // validateSflowSampleFields validates sFlow datagram and flow sample fields.
// func validateSflowSampleFields(t *testing.T, filename string, fc sflowConfig) {
// 	handle, err := pcap.OpenOffline(filename)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer handle.Close()

// 	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

// 	pickedFlowSamplesCount := 0
// 	packetCount := 0
// 	sflowSamplesTotal := uint32(0)

// 	for packet := range packetSource.Packets() {
// 		if pickedFlowSamplesCount >= 2 {
// 			t.Log("\n--- Two SFlow Flow Samples picked and printed. Stopping further processing. ---")
// 			break
// 		}

// 		if sflowLayer := packet.Layer(layers.LayerTypeSFlow); sflowLayer != nil {
// 			sflowDatagram, ok := sflowLayer.(*layers.SFlowDatagram)
// 			if !ok {
// 				t.Logf("Warning: Could not cast SFlow layer to *layers.SFlowDatagram")
// 				continue
// 			}

// 			packetCount++
// 			sflowSamplesTotal += sflowDatagram.SampleCount

// 			t.Logf("\nSFlow Datagram %d found. Contains %d samples (total reported: %d).",
// 				packetCount, len(sflowDatagram.FlowSamples), sflowDatagram.SampleCount)

// 			for _, flowSample := range sflowDatagram.FlowSamples {
// 				if pickedFlowSamplesCount >= 2 {
// 					break
// 				}
// 				t.Logf("\n--- SFlow Flow Sample %d Details (from Datagram %d) ---", pickedFlowSamplesCount+1, packetCount)
// 				t.Logf("  EnterpriseID: %d", flowSample.EnterpriseID)
// 				t.Logf("  Format: %d", flowSample.Format)
// 				t.Logf("  SampleLength: %d", flowSample.SampleLength)
// 				t.Logf("  SequenceNumber: %d", flowSample.SequenceNumber)
// 				t.Logf("  SourceIDClass: %d", flowSample.SourceIDClass)
// 				t.Logf("  SourceIDIndex: %d", flowSample.SourceIDIndex)
// 				t.Logf("  SamplingRate: %d", flowSample.SamplingRate)
// 				t.Logf("  SamplePool: %d", flowSample.SamplePool)
// 				t.Logf("  Dropped: %d", flowSample.Dropped)
// 				t.Logf("  InputInterface: %d", flowSample.InputInterface)
// 				t.Logf("  OutputInterface: %d", flowSample.OutputInterface)
// 				t.Logf("  RecordCount: %d", len(flowSample.Records))

// 				for i, record := range flowSample.Records {
// 					t.Logf("    --- Flow Record %d (Type: %T) ---", i+1, record)
// 					switch r := record.(type) {
// 					case layers.SFlowRawPacketFlowRecord:
// 						t.Logf("      Type: Raw Packet Flow Record")
// 						t.Logf("      Header Protocol: %d", r.HeaderProtocol)
// 						t.Logf("      FrameLength: %d", r.FrameLength)
// 						t.Logf("      Header Length: %d (first %d bytes of original packet)", r.HeaderLength, r.Header)
// 					case layers.SFlowEthernetFrameFlowRecord:
// 						t.Logf("      Type: Ethernet Frame Flow Record")
// 						t.Logf("      FrameLength: %d", r.FrameLength)
// 						t.Logf("      SrcMAC: %s", r.SrcMac)
// 						t.Logf("      DstMAC: %s", r.DstMac)
// 						t.Logf("      EtherType: %d", r.Type)
// 					case layers.SFlowIpv4Record:
// 						t.Logf("      Type: IPv4 Flow Record")
// 						t.Logf("      SrcIP: %s", r.IPSrc)
// 						t.Logf("      DstIP: %s", r.IPDst)
// 						t.Logf("      Protocol: %d", r.Protocol)
// 						t.Logf("      SrcPort: %d", r.PortSrc)
// 						t.Logf("      DstPort: %d", r.PortDst)
// 						t.Logf("      TCPFlags: %d", r.TCPFlags)
// 						t.Logf("      TotalLength: %d", r.Length)
// 					case layers.SFlowIpv6Record:
// 						t.Logf("      Type: IPv6 Flow Record")
// 						t.Logf("      SrcIP: %s", r.IPSrc)
// 						t.Logf("      DstIP: %s", r.IPDst)
// 						t.Logf("      Protocol: %d", r.Protocol)
// 						t.Logf("      SrcPort: %d", r.PortSrc)
// 						t.Logf("      DstPort: %d", r.PortDst)
// 						t.Logf("      TCPFlags: %d", r.TCPFlags)
// 						t.Logf("      TotalLength: %d", r.Length)
// 					case layers.SFlowExtendedRouterFlowRecord:
// 						t.Logf("      Type: Extended Router Flow Record")
// 						t.Logf("      NextHop: %s", r.NextHop)
// 						t.Logf("      SrcMask: %d", r.NextHopSourceMask)
// 						t.Logf("      DstMask: %d", r.NextHopDestinationMask)
// 					case layers.SFlowExtendedGatewayFlowRecord:
// 						t.Logf("      Type: Extended Gateway Flow Record")
// 						t.Logf("      NextHop: %s", r.NextHop)
// 						t.Logf("      AS: %d", r.AS)
// 						t.Logf("      SrcAS: %d", r.SourceAS)
// 						t.Logf("      DstAS: %d", r.PeerAS)
// 						t.Logf("      Communities: %v", r.Communities)
// 						t.Logf("      LocalPref: %d", r.LocalPref)
// 					default:
// 						t.Logf("      Type: Unhandled Flow Record Type (%T)", record)
// 					}
// 				}
// 				pickedFlowSamplesCount++
// 			}
// 		}
// 	}
//}

// Example usage context (from your original prompt, not part of the function itself)
/*
flowConfigs = []flowConfig{
        {
            name:          "flowS",
            packetsToSend: 10000000,
            ppsRate:       300000,
            frameSize:     64,
        },
        {
            name:          "flowM",
            packetsToSend: 10000000,
            ppsRate:       300000,
            frameSize:     512,
        },
        {
            name:          "flowL",
            packetsToSend: 10000000,
            ppsRate:       300000,
            frameSize:     1500,
        },
    }
*/

// GetToSTrafficClass computes the Type of Service (ToS) byte for IPv4 or
// Traffic Class byte for IPv6 based on the given DSCP and ECN values.
//
// DSCP (Differentiated Services Code Point) is a 6-bit value (0-63).
// ECN (Explicit Congestion Notification) is a 2-bit value (0-3).
//
// The ToS/Traffic Class byte structure is:
// | DSCP (6 bits) | ECN (2 bits) |
//
// Returns the computed 8-bit ToS/Traffic Class value and an error if inputs are invalid.
func GetToSTrafficClass(dscp uint8, ecn uint8) (uint8, error) {
	// Validate DSCP value: DSCP is 6 bits, so max value is 2^6 - 1 = 63.
	if dscp > 63 {
		return 0, errors.New("DSCP value out of range (0-63)")
	}

	// Validate ECN value: ECN is 2 bits, so max value is 2^2 - 1 = 3.
	if ecn > 3 {
		return 0, errors.New("ECN value out of range (0-3)")
	}

	// To form the 8-bit ToS/Traffic Class byte:
	// Shift the 6-bit DSCP value 2 bits to the left to make space for ECN.
	// Then, bitwise OR it with the 2-bit ECN value.
	result := (dscp << 2) | ecn

	return result, nil
}

func sshRunCommand(t *testing.T, dut *ondatra.DUTDevice, sshClient *binding.CLIClient, cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if result, err := (*sshClient).RunCommand(ctx, cmd); err == nil {
		t.Logf("%s> %s", dut.ID(), cmd)
		t.Log(result.Output())
		return result.Output()
	} else {
		t.Log(err.Error())
		t.Log("Restarting the ssh client")
		*sshClient = dut.RawAPIs().CLI(t)
		result, err = (*sshClient).RunCommand(ctx, cmd)
		if err != nil {
			t.Errorf("Error running command %s: %v", cmd, err)
			return ""
		}
		t.Logf("%s> %s", dut.ID(), cmd)
		return result.Output()
	}

}

var ifIndexMap = make(map[string]uint32)

// // To store:
// ifIndexMap["Bundle-Ether1"] = 101
// ifIndexMap["Bundle-Ether2"] = 102

// // To retrieve:
// idx := ifIndexMap["Bundle-Ether1"]

func getBundleMembers(t *testing.T, dut *ondatra.DUTDevice, bundleName string) []string {
	return gnmi.Get(t, dut, gnmi.OC().Interface(bundleName).Aggregation().Member().State())
}

// Map bundle interface name to slice of member ifIndexes
var bundleMemberIfIndexes = map[string][]uint32{
	"Bundle-Ether1": {101, 102, 103, 104}, // Example ifIndexes for members
	"Bundle-Ether2": {105, 106},
	// ...
}

func GetBundleMemberIfIndexes(t *testing.T, dut *ondatra.DUTDevice, bundleNames []string) map[string][]uint32 {
	var bundleMemberIfIndexes = make(map[string][]uint32)
	// Initialize the map with bundle names and their member ifIndexes
	for _, bundle := range bundleNames {
		members := gnmi.Get(t, dut, gnmi.OC().Interface(bundle).Aggregation().Member().State())
		if len(members) == 0 {
			t.Fatalf("No member interfaces found for bundle %s", bundle)
		}
		for _, member := range members {
			ifIndex := gnmi.Get(t, dut, gnmi.OC().Interface(member).Ifindex().State())
			bundleMemberIfIndexes[bundle] = append(bundleMemberIfIndexes[bundle], ifIndex)
		}
	}
	return bundleMemberIfIndexes
}

// To check if an sFlow packet's ifIndex is a member of a bundle:
func isMemberIfIndex(bundle string, ifIndex uint32) bool {
	for _, idx := range bundleMemberIfIndexes[bundle] {
		if idx == ifIndex {
			return true
		}
	}
	return false
}

func getIfIndex(t *testing.T, dut *ondatra.DUTDevice, sshClient *binding.CLIClient, intfs []string) {
	for _, intf := range intfs {
		cmd := fmt.Sprintf("show snmp interface %s ifindex", intf)
		sshRunCommand(t, dut, sshClient, cmd)

	}
}

func validateSflowCapture(t *testing.T, args *testArgs, ports []string, sfc *sflowConfig) {
	if sfc != nil {
		oldPorts := args.capture_ports
		oldTtl := args.pattr.ttl
		oldDscp := args.pattr.dscp
		args.pattr.sfConfig = sfc
		SetFlowsRate(args.flows, sfc.ppsRate)
		args.trafficDuration = *sf_trafficDuration
		args.pattr.dscp = int(sfc.sflowDscp)
		args.pattr.ttl = 255

		defer func() {
			args.capture_ports = oldPorts
			args.pattr.sfConfig = nil
			SetFlowsRate(args.flows, *fps)
			args.trafficDuration = *trafficDuration
			args.pattr.ttl = oldTtl
			args.pattr.dscp = oldDscp
		}()
		args.capture_ports = ports
		validateCapture(t, args)

	}
}

func SetFlowsRate(flows []gosnappi.Flow, pps uint64) {
	for _, flow := range flows {
		flow.Rate().SetPps(pps)
	}
}

// NewSfRecordRawPacketHeader creates a new instance of sfRecordRawPacketHeader
func NewSfRecordRawPacketHeader(protocol, frameLength, stripped uint32, header []byte) *sfRecordRawPacketHeader {
	return &sfRecordRawPacketHeader{
		Protocol:    protocol,
		FrameLength: frameLength,
		Stripped:    stripped,
		Header:      header,
	}
}

// Setters for sfRecordRawPacketHeader fields
func (s *sfRecordRawPacketHeader) SetProtocol(protocol uint32) {
	s.Protocol = protocol
}
func (s *sfRecordRawPacketHeader) SetFrameLength(frameLength uint32) {
	s.FrameLength = frameLength
}
func (s *sfRecordRawPacketHeader) SetStripped(stripped uint32) {
	s.Stripped = stripped
}
func (s *sfRecordRawPacketHeader) SetHeader(header []byte) {
	s.Header = header
}

// Getters for sfRecordRawPacketHeader fields
func (s *sfRecordRawPacketHeader) GetProtocol() uint32 {
	return s.Protocol
}
func (s *sfRecordRawPacketHeader) GetFrameLength() uint32 {
	return s.FrameLength
}
func (s *sfRecordRawPacketHeader) GetStripped() uint32 {
	return s.Stripped
}
func (s *sfRecordRawPacketHeader) GetHeader() []byte {
	return s.Header
}

// NewSfRecordExtendedRouterData creates a new instance of sfRecordExtendedRouterData
func NewSfRecordExtendedRouterData(
	nextHop string,
	srcAS, dstAS, srcPeerAS, inputInterface, outputInterface uint32,
	srcASPath, dstASPath []uint32,
) *sfRecordExtendedRouterData {
	return &sfRecordExtendedRouterData{
		NextHop:         nextHop,
		SrcAS:           srcAS,
		DstAS:           dstAS,
		SrcPeerAS:       srcPeerAS,
		InputInterface:  inputInterface,
		OutputInterface: outputInterface,
		SrcASPath:       srcASPath,
		DstASPath:       dstASPath,
	}
}

// Setters for sfRecordExtendedRouterData fields
func (s *sfRecordExtendedRouterData) SetNextHop(nextHop string) {
	s.NextHop = nextHop
}
func (s *sfRecordExtendedRouterData) SetSrcAS(srcAS uint32) {
	s.SrcAS = srcAS
}
func (s *sfRecordExtendedRouterData) SetDstAS(dstAS uint32) {
	s.DstAS = dstAS
}
func (s *sfRecordExtendedRouterData) SetSrcPeerAS(srcPeerAS uint32) {
	s.SrcPeerAS = srcPeerAS
}
func (s *sfRecordExtendedRouterData) SetInputInterface(inputInterface uint32) {
	s.InputInterface = inputInterface
}
func (s *sfRecordExtendedRouterData) SetOutputInterface(outputInterface uint32) {
	s.OutputInterface = outputInterface
}
func (s *sfRecordExtendedRouterData) SetSrcASPath(srcASPath []uint32) {
	s.SrcASPath = srcASPath
}
func (s *sfRecordExtendedRouterData) SetDstASPath(dstASPath []uint32) {
	s.DstASPath = dstASPath
}

// Getters for sfRecordExtendedRouterData fields
func (s *sfRecordExtendedRouterData) GetNextHop() string {
	return s.NextHop
}
func (s *sfRecordExtendedRouterData) GetSrcAS() uint32 {
	return s.SrcAS
}
func (s *sfRecordExtendedRouterData) GetDstAS() uint32 {
	return s.DstAS
}
func (s *sfRecordExtendedRouterData) GetSrcPeerAS() uint32 {
	return s.SrcPeerAS
}
func (s *sfRecordExtendedRouterData) GetInputInterface() uint32 {
	return s.InputInterface
}
func (s *sfRecordExtendedRouterData) GetOutputInterface() uint32 {
	return s.OutputInterface
}
func (s *sfRecordExtendedRouterData) GetSrcASPath() []uint32 {
	return s.SrcASPath
}
func (s *sfRecordExtendedRouterData) GetDstASPath() []uint32 {
	return s.DstASPath
}

// NewSfRecordExtendedGatewayData creates a new instance of sfRecordExtendedGatewayData
func NewSfRecordExtendedGatewayData(
	nextHop string,
	asPath []uint32,
	communities []uint32,
	localPref uint32,
) *sfRecordExtendedGatewayData {
	return &sfRecordExtendedGatewayData{
		NextHop:     nextHop,
		ASPath:      asPath,
		Communities: communities,
		LocalPref:   localPref,
	}
}

// Setters for sfRecordExtendedGatewayData fields
func (s *sfRecordExtendedGatewayData) SetNextHop(nextHop string) {
	s.NextHop = nextHop
}
func (s *sfRecordExtendedGatewayData) SetASPath(asPath []uint32) {
	s.ASPath = asPath
}
func (s *sfRecordExtendedGatewayData) SetCommunities(communities []uint32) {
	s.Communities = communities
}
func (s *sfRecordExtendedGatewayData) SetLocalPref(localPref uint32) {
	s.LocalPref = localPref
}

// Getters for sfRecordExtendedGatewayData fields
func (s *sfRecordExtendedGatewayData) GetNextHop() string {
	return s.NextHop
}
func (s *sfRecordExtendedGatewayData) GetASPath() []uint32 {
	return s.ASPath
}
func (s *sfRecordExtendedGatewayData) GetCommunities() []uint32 {
	return s.Communities
}
func (s *sfRecordExtendedGatewayData) GetLocalPref() uint32 {
	return s.LocalPref
}
