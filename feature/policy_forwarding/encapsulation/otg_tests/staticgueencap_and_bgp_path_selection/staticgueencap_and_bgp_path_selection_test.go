package staticgueencap_and_bgp_path_selection_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Constants for address families, AS numbers, and protocol settings.
const (
	plenIPv4p2p   = 31
	plenIPv6p2p   = 127
	plenIPv4lo    = 32
	plenIPv6lo    = 128
	plenUserV4    = 24
	plenUserV6    = 64
	dutAS         = 65501
	ateIBGPAS     = 65501 // For iBGP with DUT
	ateEBGPAS     = 65502 // For eBGP with DUT
	isisInstance  = "DEFAULT"
	isisSysID1    = "640000000001"
	isisSysID2    = "640000000002"
	isisAreaAddr  = "49.0001"
	dutSysID      = "1920.0000.2001"
	isisMetric    = 10
	ibgpPeerGroup = "IBGP-PEERS"
	ebgpPeerGroup = "EBGP-PEERS"
	udpEncapPort  = 6080
	nhgTTL        = 64

	// Static and GUE address
	nexthopGroupName   = "GUE-NHG"
	nexthopGroupNameV6 = "GUE-NHGv6"
	guePolicyName      = "GUE-Policy"

	totalPackets = 50000
	trafficPps   = 1000
	sleepTime    = time.Duration(totalPackets/trafficPps) * time.Second
)

var (
	// DUT Port 1 <-> ATE Port 1 connection
	dutPort1 = &attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    "192.0.2.3",
		IPv6:    "2001:db8:1::3",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}
	atePort1 = &attrs.Attributes{
		Name:    "port1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8:1::2",
		MAC:     "02:01:01:01:01:01",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}

	// DUT Port 2 <-> ATE Port 2 connection
	dutPort2 = &attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    "192.0.2.7",
		IPv6:    "2001:db8:1::7",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}
	atePort2 = &attrs.Attributes{
		Name:    "port2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8:1::6",
		MAC:     "02:02:02:02:02:02",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}

	// DUT Port 3 <-> ATE Port 3 connection
	dutPort3 = &attrs.Attributes{
		Desc:    "DUT to ATE Port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8:1::9",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}
	atePort3 = &attrs.Attributes{
		Name:    "port3",
		IPv4:    "192.0.2.8",
		IPv6:    "2001:db8:1::8",
		MAC:     "02:03:03:03:03:03",
		IPv4Len: plenIPv4p2p,
		IPv6Len: plenIPv6p2p,
	}

	// DUT loopback 0 ($DUT_lo0)
	dutloopback0 = &attrs.Attributes{
		Desc:    "DUT Loopback 0",
		IPv4:    "203.0.113.10",
		IPv6:    "2001:db8::203:0:113:10",
		IPv4Len: plenIPv4lo,
		IPv6Len: plenIPv6lo,
	}

	// ATE Port1 user prefixes
	ate1UserPrefixesV4     = "198.61.100.1"
	ate1UserPrefixesV6     = "2001:db8:100:1::"
	ate1UserPrefixesCount  = uint32(5)
	ate1UserPrefixesV4List = iputil.GenerateIPs(ate1UserPrefixesV4+"/24", int(ate1UserPrefixesCount))
	ate1UserPrefixesV6List = iputil.GenerateIPv6(ate1UserPrefixesV6+"/64", uint64(ate1UserPrefixesCount))

	// $ATE2_INTERNAL - Prefixes to be advertised by ATE Port2 IBGP/ ATE2_C
	ate2InternalPrefixesV4     = "198.71.100.1"
	ate2InternalPrefixesV6     = "2001:db8:200:1::"
	ate2InternalPrefixCount    = uint32(5)
	ate2InternalPrefixesV4List = iputil.GenerateIPs(ate2InternalPrefixesV4+"/24", int(ate2InternalPrefixCount))
	ate2InternalPrefixesV6List = iputil.GenerateIPv6(ate2InternalPrefixesV6+"/64", uint64(ate2InternalPrefixCount))

	// ATE Port3 or ATE2 Port3 bgp prefixes
	bgpInternalTE11 = &attrs.Attributes{
		Name:    "ate2InternalTE11",
		IPv4:    "198.18.11.1",
		IPv4Len: 32,
	}
	bgpInternalTE10 = &attrs.Attributes{
		Name:    "ate2InternalTE10",
		IPv4:    "198.18.10.1",
		IPv4Len: 32,
	}

	// DUT Tunnel Configurations EP 10, 11
	dutTE11 = &attrs.Attributes{
		Desc:    "DUT Tunnel Endpoint 11",
		IPv4:    "198.51.100.10",
		IPv4Len: 32,
	}

	dutTE10 = &attrs.Attributes{
		Desc:    "DUT Tunnel Endpoint 10",
		IPv4:    "198.51.100.13",
		IPv4Len: 32,
	}

	// ATE Port2 C.IBGP ---> DUT connected via Pseudo Protocol Next-Hops
	ate2ppnh1 = &attrs.Attributes{Name: "ate2ppnh1", IPv6: "2001:db8:2::1", IPv6Len: plenIPv6lo}
	ate2ppnh2 = &attrs.Attributes{Name: "ate2ppnh2", IPv6: "2001:db8:3::1", IPv6Len: plenIPv6lo}

	ate2ppnhPrefix = "2001:db8:2::0/128"

	atePorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
	}

	loopbackIntfName string

	dscpValue = map[string]uint32{
		"BE1": 0,
		"AF1": 10,
		"AF2": 18,
		"AF3": 26,
		"AF4": 34,
	}

	atePort1RouteV4 = "v4-user-routes"
	atePort1RouteV6 = "v6-user-routes"

	atePort2RoutesV4   = "v4-internal-routes"
	atePort2RoutesV6   = "v6-internal-routes"
	atePort2RoutesTE10 = "v4-TE10-routes"
	atePort2RoutesTE11 = "v4-TE11-routes"

	ateCPort2Routes1V6 = "v6-internal-routes-1"
	ateCPort2Routes2V6 = "v6-internal-routes-2"
	ateCPort2Routes1V4 = "v4-internal-routes-1"
	ateCPort2Routes2V4 = "v4-internal-routes-2"

	ateCPort3RoutesV6 = "internal-routesV6-1-port3"

	trafficFlowData = []*trafficFlow{
		{
			name:           "flowSet1-v4-1",
			srcDevice:      []string{atePort1RouteV4},
			dstDevice:      []string{atePort2RoutesV4},
			srcAddr:        []string{ate1UserPrefixesV4List[0]},
			dstAddr:        []string{ate2InternalPrefixesV4List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      true,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet1-v6-1",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{atePort2RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[0]},
			dstAddr:        []string{ate2InternalPrefixesV6List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet1-v4-2",
			srcDevice:      []string{atePort1RouteV4},
			dstDevice:      []string{atePort2RoutesV4},
			srcAddr:        []string{ate1UserPrefixesV4List[1]},
			dstAddr:        []string{ate2InternalPrefixesV4List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      true,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet1-v6-2",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{atePort2RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[1]},
			dstAddr:        []string{ate2InternalPrefixesV6List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet1-v4-3",
			srcDevice:      []string{atePort1RouteV4},
			dstDevice:      []string{atePort2RoutesV4},
			srcAddr:        []string{ate1UserPrefixesV4List[2]},
			dstAddr:        []string{ate2InternalPrefixesV4List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      true,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet1-v6-3",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{atePort2RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[2]},
			dstAddr:        []string{ate2InternalPrefixesV6List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet2-v4-1",
			srcDevice:      []string{atePort1RouteV4},
			dstDevice:      []string{atePort2RoutesV4},
			srcAddr:        []string{ate1UserPrefixesV4List[3]},
			dstAddr:        []string{ate2InternalPrefixesV4List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      true,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
		{
			name:           "flowSet2-v6-1",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{atePort2RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[3]},
			dstAddr:        []string{ate2InternalPrefixesV6List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
		{
			name:           "flowSet2-v4-2",
			srcDevice:      []string{atePort1RouteV4},
			dstDevice:      []string{atePort2RoutesV4},
			srcAddr:        []string{ate1UserPrefixesV4List[4]},
			dstAddr:        []string{ate2InternalPrefixesV4List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      true,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
		{
			name:           "flowSet2-v6-2",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{atePort2RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[4]},
			dstAddr:        []string{ate2InternalPrefixesV6List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
		{
			name:           "flowSet3-v4-1",
			srcDevice:      []string{atePort3.Name + ".IPv4"},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[0]},
			dstAddr:        []string{ate1UserPrefixesV4List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      true,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet3-v6-1",
			srcDevice:      []string{atePort3.Name + ".IPv6"},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[0]},
			dstAddr:        []string{ate1UserPrefixesV6List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      false,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet3-v4-2",
			srcDevice:      []string{atePort3.Name + ".IPv4"},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[1]},
			dstAddr:        []string{ate1UserPrefixesV4List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      true,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet3-v6-2",
			srcDevice:      []string{atePort3.Name + ".IPv6"},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[1]},
			dstAddr:        []string{ate1UserPrefixesV6List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      false,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet3-v4-3",
			srcDevice:      []string{atePort3.Name + ".IPv4"},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[2]},
			dstAddr:        []string{ate1UserPrefixesV4List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      true,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet3-v6-3",
			srcDevice:      []string{atePort3.Name + ".IPv6"},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[2]},
			dstAddr:        []string{ate1UserPrefixesV6List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      false,
			tunnelEndpoint: dutTE11.IPv4,
		},
		{
			name:           "flowSet4-v4-4",
			srcDevice:      []string{atePort3.Name + ".IPv4"},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[3]},
			dstAddr:        []string{ate1UserPrefixesV4List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      true,
			tunnelEndpoint: dutTE10.IPv4,
		},
		{
			name:           "flowSet4-v6-4",
			srcDevice:      []string{atePort3.Name + ".IPv6"},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[3]},
			dstAddr:        []string{ate1UserPrefixesV6List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      false,
			tunnelEndpoint: dutTE10.IPv4,
		},
		{
			name:           "flowSet4-v4-5",
			srcDevice:      []string{atePort3.Name + ".IPv4"},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[4]},
			dstAddr:        []string{ate1UserPrefixesV4List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      true,
			tunnelEndpoint: dutTE10.IPv4,
		},
		{
			name:           "flowSet4-v6-5",
			srcDevice:      []string{atePort3.Name + ".IPv6"},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[4]},
			dstAddr:        []string{ate1UserPrefixesV6List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      false,
			tunnelEndpoint: dutTE10.IPv4,
		},
		{
			name:           "flowSet5-v4-1",
			srcDevice:      []string{atePort2RoutesV4},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[0]},
			dstAddr:        []string{ate1UserPrefixesV4List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      true,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v6-1",
			srcDevice:      []string{atePort2RoutesV6},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[0]},
			dstAddr:        []string{ate1UserPrefixesV6List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      false,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v4-2",
			srcDevice:      []string{atePort2RoutesV4},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[1]},
			dstAddr:        []string{ate1UserPrefixesV4List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      true,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v6-2",
			srcDevice:      []string{atePort2RoutesV6},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[1]},
			dstAddr:        []string{ate1UserPrefixesV6List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      false,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v4-3",
			srcDevice:      []string{atePort2RoutesV4},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[2]},
			dstAddr:        []string{ate1UserPrefixesV4List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      true,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v6-3",
			srcDevice:      []string{atePort2RoutesV6},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[2]},
			dstAddr:        []string{ate1UserPrefixesV6List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      false,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v4-4",
			srcDevice:      []string{atePort2RoutesV4},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[3]},
			dstAddr:        []string{ate1UserPrefixesV4List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      true,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v6-4",
			srcDevice:      []string{atePort2RoutesV6},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[3]},
			dstAddr:        []string{ate1UserPrefixesV6List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      false,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v4-5",
			srcDevice:      []string{atePort2RoutesV4},
			dstDevice:      []string{atePort1RouteV4},
			srcAddr:        []string{ate2InternalPrefixesV4List[4]},
			dstAddr:        []string{ate1UserPrefixesV4List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      true,
			tunnelEndpoint: "",
		},
		{
			name:           "flowSet5-v6-5",
			srcDevice:      []string{atePort2RoutesV6},
			dstDevice:      []string{atePort1RouteV6},
			srcAddr:        []string{ate2InternalPrefixesV6List[4]},
			dstAddr:        []string{ate1UserPrefixesV6List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      false,
			tunnelEndpoint: "",
		},
	}
)

type BGPRib struct {
	prefix    string
	origin    string
	pathId    int
	isPresent bool
}

type isisConfig struct {
	port  string
	level oc.E_Isis_LevelType
}

type bgpNbr struct {
	peerGrpName string
	nbrIp       string
	srcIp       string
	peerAs      uint32
	isV4        bool
}

type trafficFlow struct {
	name           string
	srcDevice      []string
	dstDevice      []string
	srcAddr        []string
	dstAddr        []string
	trafficPps     uint64
	packetSize     uint32
	dscp           uint8
	v4Traffic      bool
	tunnelEndpoint string
}

type flowGroupData struct {
	Flows    []gosnappi.Flow
	Endpoint string
}

var flowGroups = make(map[string]flowGroupData)

// configureDUT configures interfaces, BGP, IS-IS, and static tunnel routes on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))
	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(p3, dutPort3, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}

	configureLoopback(t, dut)

	isisConf := []*isisConfig{
		{port: p1.Name(), level: oc.Isis_LevelType_LEVEL_2},
		{port: p2.Name(), level: oc.Isis_LevelType_LEVEL_2},
	}

	configureISIS(t, dut, isisConf)

	nbrs := []*bgpNbr{
		{peerAs: ateIBGPAS, nbrIp: atePort1.IPv4, isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate1", srcIp: dutloopback0.IPv4},
		{peerAs: ateIBGPAS, nbrIp: atePort1.IPv6, isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate1", srcIp: dutloopback0.IPv6},
		{peerAs: ateIBGPAS, nbrIp: atePort2.IPv4, isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate2", srcIp: dutloopback0.IPv4},
		{peerAs: ateIBGPAS, nbrIp: atePort2.IPv6, isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate2", srcIp: dutloopback0.IPv6},
		{peerAs: ateEBGPAS, nbrIp: atePort3.IPv4, isV4: true, peerGrpName: ebgpPeerGroup, srcIp: dutPort3.IPv4},
		{peerAs: ateEBGPAS, nbrIp: atePort3.IPv6, isV4: false, peerGrpName: ebgpPeerGroup, srcIp: dutPort3.IPv6},
	}

	bgpCreateNbr(t, dutAS, dut, nbrs)

	// Configure static routes from PNH to nexthopgroup
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          ate2ppnhPrefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nexthopGroupName),
		},
	}

	cfgplugins.NewStaticRouteNextHopGroupCfg(t, b, sV4, dut, nexthopGroupName)

}

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

func configureLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	// Configure interface loopback
	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutloopback0.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutloopback0.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutloopback0.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutloopback0.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutloopback0.IPv6)
	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, isisIntf []*isisConfig) {
	// Configure IS-IS protocol on port1 and port2
	root := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isisProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)

	isisProtocol.SetEnabled(true)
	isis := isisProtocol.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.SetInstance(isisInstance)
	}

	// Configure Global ISIS settings
	globalISIS.SetNet([]string{fmt.Sprintf("%s.%s.00", isisAreaAddr, dutSysID)})
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.SetEnabled(true)
	}

	for _, isisPort := range isisIntf {
		intf := isis.GetOrCreateInterface(isisPort.port)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.SetEnabled(true)
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			intf.GetOrCreateLevel(1).SetEnabled(false)
		} else {
			intf.GetOrCreateLevel(2).SetEnabled(true)
		}
		globalISIS.LevelCapability = isisPort.level
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			intf.Af = nil
		}
	}

	// Push ISIS configuration to DUT
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), isisProtocol)

}

func bgpCreateNbr(t *testing.T, localAs uint32, dut *ondatra.DUTDevice, bgpNbr []*bgpNbr) {
	localAddressLeaf := ""
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutloopback0.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	for _, nbr := range bgpNbr {
		pg1 := bgp.GetOrCreatePeerGroup(nbr.peerGrpName)
		pg1.PeerAs = ygot.Uint32(nbr.peerAs)

		bgpNbr := bgp.GetOrCreateNeighbor(nbr.nbrIp)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrpName)
		bgpNbr.PeerAs = ygot.Uint32(nbr.peerAs)
		bgpNbr.Enabled = ygot.Bool(true)
		bgpNbrT := bgpNbr.GetOrCreateTransport()

		localAddressLeaf = nbr.srcIp

		if dut.Vendor() == ondatra.CISCO {
			localAddressLeaf = dutloopback0.Name
		}
		bgpNbrT.LocalAddress = ygot.String(localAddressLeaf)
		af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
	}

	gnmi.Replace(t, dut, dutConfPath.Config(), niProto)
}

func configureOTG(t *testing.T, otg *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	// Configure OTG Port1
	iDutDev := configureInterfaces(otgConfig, "port1")

	// Enable ISIS and BGP Protocols on port 1.
	isisDut := iDutDev.Isis().SetName("ISIS1").SetSystemId(isisSysID1)
	isisDut.Basic().SetIpv4TeRouterId(atePort1.IPv4).SetHostname(isisDut.Name()).SetLearnedLspFilter(true)
	isisDut.Interfaces().Add().SetEthName(iDutDev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt1").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	iDutBgp := iDutDev.Bgp().SetRouterId(atePort1.IPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(iDutDev.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// Advertise user prefixes on port 1
	v4routes := iDutBgp4Peer.V4Routes().Add().SetName(atePort1RouteV4)
	v4routes.Addresses().Add().SetAddress(ate1UserPrefixesV4).SetStep(1).SetPrefix(24).SetCount(ate1UserPrefixesCount)

	// Advertise user prefixes v6 on port 1
	v6routes := iDutBgp4Peer.V6Routes().Add().SetName(atePort1RouteV6)
	v6routes.Addresses().Add().SetAddress(ate1UserPrefixesV6).SetStep(1).SetPrefix(64).SetCount(ate1UserPrefixesCount)
	v6routes.SetNextHopIpv6Address(atePort1.IPv6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)

	// Configure OTG Port2
	iDut2Dev := configureInterfaces(otgConfig, "port2")

	// Enable ISIS and BGP Protocols on port 2
	isis2Dut := iDut2Dev.Isis().SetName("ISIS2").SetSystemId(isisSysID2)
	isis2Dut.Basic().SetIpv4TeRouterId(atePort2.IPv4).SetHostname(isis2Dut.Name()).SetLearnedLspFilter(true)
	isis2Dut.Interfaces().Add().SetEthName(iDut2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Configure IBGP Peer on port2
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(atePort2.IPv4)
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut2Bgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Advertise prefixes from IBGP Peer
	iDut2Bgpv4routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesV4)
	iDut2Bgpv4routes.Addresses().Add().SetAddress(ate2InternalPrefixesV4).SetStep(1).SetPrefix(24).SetCount(ate2InternalPrefixCount)

	iDut2Bgpv6routes := iDut2Bgp4Peer.V6Routes().Add().SetName(atePort2RoutesV6)
	iDut2Bgpv6routes.SetNextHopIpv6Address(atePort2.IPv6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	iDut2Bgpv6routes.Addresses().Add().SetAddress(ate2InternalPrefixesV6).SetStep(1).SetPrefix(64).SetCount(ate1UserPrefixesCount)

	iDut2BgpTe10Routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesTE10)
	iDut2BgpTe10Routes.Addresses().Add().SetAddress(bgpInternalTE10.IPv4).SetPrefix(uint32(bgpInternalTE10.IPv4Len)).SetCount(1)

	iDut2BgpTe11Routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesTE11)
	iDut2BgpTe11Routes.Addresses().Add().SetAddress(bgpInternalTE11.IPv4).SetPrefix(uint32(bgpInternalTE11.IPv4Len)).SetCount(1)

	// Configure IBGP_C on port 2
	ate2CBgpv6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()).Peers().Add().SetName(atePort2.Name + ".CBGP6.peer")
	ate2CBgpv6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	ate2CBgpv6Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	ate2CBgpv6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// routes adevertised from C-BGPv6
	v6routes2a := ate2CBgpv6Peer.V6Routes().Add().SetName(ateCPort2Routes1V6)
	v6routes2a.SetNextHopIpv6Address(ate2ppnh1.IPv6).AddPath().SetPathId(1)
	v6routes2a.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
	v6routes2a.Addresses().Add().SetAddress(ate2InternalPrefixesV6).SetPrefix(64).SetCount(3)

	v6routes2b := ate2CBgpv6Peer.V6Routes().Add().SetName(ateCPort2Routes2V6)
	v6routes2b.SetNextHopIpv6Address(ate2ppnh2.IPv6)
	v6routes2b.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
	v6routes2b.Addresses().Add().SetAddress(ate2InternalPrefixesV6List[3]).SetPrefix(64).SetCount(2)

	v4routes2a := ate2CBgpv6Peer.V4Routes().Add().SetName(ateCPort2Routes1V4)
	v4routes2a.SetNextHopIpv6Address(ate2ppnh1.IPv6).AddPath().SetPathId(1)
	v4routes2a.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
	v4routes2a.Addresses().Add().SetAddress(ate2InternalPrefixesV4).SetPrefix(24).SetCount(3)

	v4routes2b := ate2CBgpv6Peer.V4Routes().Add().SetName(ateCPort2Routes2V4)
	v4routes2b.SetNextHopIpv6Address(ate2ppnh2.IPv6)
	v4routes2b.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
	v4routes2b.Addresses().Add().SetAddress(ate2InternalPrefixesV4List[3]).SetPrefix(24).SetCount(2)

	// Configure OTG Port3
	iDut3Dev := configureInterfaces(otgConfig, "port3")

	ate3Bgp := iDut3Dev.Bgp().SetRouterId(atePort3.IPv4)

	ate3Bgpv4Peer := ate3Bgp.Ipv4Interfaces().Add().SetIpv4Name(atePort3.Name + ".IPv4").Peers().Add().SetName("ate3.bgp4.peer")
	ate3Bgpv4Peer.SetPeerAddress(dutPort3.IPv4).SetAsNumber(ateEBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP).LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ate3Bgpv6Peer := ate3Bgp.Ipv6Interfaces().Add().SetIpv6Name(atePort3.Name + ".IPv6").Peers().Add().SetName("ate3.bgp6.peer")
	ate3Bgpv6Peer.SetPeerAddress(dutPort3.IPv6).SetAsNumber(ateEBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP).LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ebgpRoutes := ate3Bgpv4Peer.V4Routes().Add().SetName("ebgp4-te10-routes")
	ebgpRoutes.Addresses().Add().SetAddress(bgpInternalTE10.IPv4).SetPrefix(uint32(30))

	ebgpRoutes11 := ate3Bgpv4Peer.V4Routes().Add().SetName("ebgp4-te11-routes")
	ebgpRoutes11.Addresses().Add().SetAddress(bgpInternalTE11.IPv4).SetPrefix(uint32(30))

	// routes adevertised from C-BGPv6
	v6routes3a := ate3Bgpv6Peer.V6Routes().Add().SetName(ateCPort3RoutesV6)
	v6routes3a.SetNextHopIpv6Address(ate2ppnh1.IPv6).AddPath().SetPathId(1)
	v6routes3a.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
	v6routes3a.Addresses().Add().SetAddress(ate2InternalPrefixesV6).SetPrefix(64).SetCount(5)

	return otgConfig

}

func configureInterfaces(otgConfig gosnappi.Config, port string) gosnappi.Device {
	portAttr := atePorts[port]
	dutAttr := dutPorts[port]

	portObj := otgConfig.Ports().Add().SetName(port)
	iDutDev := otgConfig.Devices().Add().SetName(portAttr.Name)
	iDutEth := iDutDev.Ethernets().Add().SetName(portAttr.Name + ".Eth").SetMac(portAttr.MAC)
	iDutEth.Connection().SetPortName(portObj.Name())
	iDutIpv4 := iDutEth.Ipv4Addresses().Add().SetName(portAttr.Name + ".IPv4")
	iDutIpv4.SetAddress(portAttr.IPv4).SetGateway(dutAttr.IPv4).SetPrefix(uint32(portAttr.IPv4Len))
	iDutIpv6 := iDutEth.Ipv6Addresses().Add().SetName(portAttr.Name + ".IPv6")
	iDutIpv6.SetAddress(portAttr.IPv6).SetGateway(dutAttr.IPv6).SetPrefix(uint32(portAttr.IPv6Len))

	return iDutDev
}

func configureTrafficFlows(trafficFlowData []*trafficFlow) {
	flowSetNum := regexp.MustCompile(`^flowSet(\d+)`)

	for _, trafficFlow := range trafficFlowData {
		flow := createFlow(trafficFlow)
		flowSet := flowSetNum.FindStringSubmatch(trafficFlow.name)[0]

		fg, exists := flowGroups[flowSet]
		if !exists {
			fg = flowGroupData{
				Endpoint: trafficFlow.tunnelEndpoint,
			}
		}
		fg.Flows = append(fg.Flows, flow)
		flowGroups[flowSet] = fg
	}
}

func createFlow(trafficFlow *trafficFlow) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(trafficFlow.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames(trafficFlow.srcDevice).SetRxNames(trafficFlow.dstDevice)
	flow.Rate().SetPps(trafficFlow.trafficPps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(trafficFlow.packetSize))

	flow.Packet().Add().Ethernet()

	if trafficFlow.v4Traffic {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValues(trafficFlow.srcAddr)
		v4.Dst().SetValues(trafficFlow.dstAddr)
		v4.Priority().Dscp().Phb().SetValue(uint32(trafficFlow.dscp))
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValues(trafficFlow.srcAddr)
		v6.Dst().SetValues(trafficFlow.dstAddr)
		v6.TrafficClass().SetValue(uint32(trafficFlow.dscp))
	}

	return flow
}

func withdrawBGPRoutes(t *testing.T, routeNames []string) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

}

func advertiseBGPRoutes(t *testing.T, routeNames []string) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
	otg.SetControlState(t, cs)

}

func validateTrafficLoss(t *testing.T, otgConfig *otg.OTG, flowName []string) {
	for _, flow := range flowName {
		outPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow).Counters().InPkts().State()))
		t.Logf("Flow %s: outPkts: %v, inPkts: %v", flow, outPkts, inPkts)
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
			t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
		}
	}
}

func validatePrefixes(t *testing.T, dut *ondatra.DUTDevice, neighborIP string, isV4 bool, PfxRcd, PfxSent uint32) {
	t.Helper()

	t.Logf("Validate prefixes for %s. Expecting prefix received %v", neighborIP, PfxRcd)
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	if isV4 {
		ipv4Pfx := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes](t, dut, bgpPath.Neighbor(neighborIP).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().State())
		if PfxRcd != ipv4Pfx.GetReceived() {
			t.Errorf("Received Prefixes - got: %v, want: %v", ipv4Pfx.GetReceived(), PfxRcd)
		}
		if PfxSent != ipv4Pfx.GetSent() {
			t.Errorf("Sent Prefixes - got: %v, want: %v", ipv4Pfx.GetSent(), PfxSent)
		}
	} else {
		ipv6Pfx := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes](t, dut, bgpPath.Neighbor(neighborIP).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().State())
		if PfxRcd != ipv6Pfx.GetReceived() {
			t.Errorf("Received Prefixes - got: %v, want: %v", ipv6Pfx.GetReceived(), PfxRcd)
		}
		if PfxSent != ipv6Pfx.GetSent() {
			t.Errorf("Sent Prefixes - got: %v, want: %v", ipv6Pfx.GetSent(), PfxSent)
		}
	}
}

func enableCapture(t *testing.T, otg *otg.OTG, otgConfig gosnappi.Config, portName string) {
	otgConfig.Captures().Add().SetName(portName).SetPortNames([]string{portName}).SetFormat(gosnappi.CaptureFormat.PCAP)
}

func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	capture, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capture.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capture.Close()

	return capture.Name()
}

func validatePackets(t *testing.T, filename string, protocolType string, outertos, innertos, outerttl uint8, outerDstIP string, outerPacket bool) {
	var packetCount uint32 = 0

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		packetCount += 1
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("Outer IP layer not found %d", ipLayer)
			return
		}

		udpLayer := packet.Layer(layers.LayerTypeUDP)
		udp, ok := udpLayer.(*layers.UDP)
		if !ok || udp == nil {
			t.Error("GUE layer not found")
			return
		} else {
			if udp.DstPort == udpEncapPort {
				t.Log("Got the encapsulated GUE layer")
			}

			if outerPacket {
				validateOuterPacket(t, ipOuterLayer, outertos, outerttl, outerDstIP)
			}

			var gotInnerPacketTOS uint8

			switch protocolType {
			case "ipv4":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv4, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv4)
				if ipLayer == nil {
					t.Errorf("Inner layer of type %s not found", protocolType)
					return
				}
				ip, _ := ipLayer.(*layers.IPv4)
				gotInnerPacketTOS = ip.TOS >> 2

				if gotInnerPacketTOS == innertos {
					t.Logf("TOS matched: expected TOS %v, got TOS %v", innertos, gotInnerPacketTOS)
				} else {
					t.Errorf("TOS mismatch: expected TOS %v, got TOS %v", innertos, gotInnerPacketTOS)
				}
			case "ipv6":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv6, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv6)
				if ipLayer == nil {
					t.Errorf("Inner layer of type %s not found", protocolType)
					return
				}
				// ip, _ := ipLayer.(*layers.IPv6)
				// TODO:
				// gotInnerPacketTOS = ip.TrafficClass
			}
		}
		break
	}

}

func validateOuterPacket(t *testing.T, outerPacket *layers.IPv4, tos, ttl uint8, dstIp string) {

	outerttl := outerPacket.TTL
	outerDSCP := outerPacket.TOS >> 2
	outerDstIp := outerPacket.DstIP.String()

	if dstIp != "" {
		if outerDstIp == dstIp {
			t.Logf("Encapsulted with tunnel destination: expected dstIP %s, got %s", dstIp, outerDstIp)
		} else {
			t.Errorf("Not receievd encapsulted with tunnel destination: expected dstIP %s, got %s", dstIp, outerDstIp)
		}
	}

	if ttl != 0 {
		if outerttl == ttl {
			t.Logf("Outer TTL matched: expected ttl %d, got ttl %d", ttl, outerttl)
		} else {
			t.Errorf("Outer TTL mismatch: expected ttl %d, got ttl %d", ttl, outerttl)
		}
	}
	if outerDSCP == tos {
		t.Logf("Outer TOS matched: expected TOS %v, got TOS %v", tos, outerDSCP)
	} else {
		t.Errorf("Outer TOS mismatch: expected TOS %v, got TOS %v", tos, outerDSCP)
	}

}

func validateOutCounters(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG) {
	var totalTxFromATE uint64

	flows := otg.FetchConfig(t).Flows().Items()
	for _, flow := range flows {
		txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())

		totalTxFromATE += txPkts
	}

	dutOutCounters := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().State()).GetOutUnicastPkts()

	expectedTotalTraffic := uint64(totalPackets * len(flows))
	if totalTxFromATE > 0 {
		if float64(dutOutCounters) < float64(totalTxFromATE)*0.98 {
			t.Errorf("DUT Counters is significantly less than ATE Tx (%d). Recieved: %d, Expected approx %d.", totalTxFromATE, dutOutCounters, expectedTotalTraffic)
		}
	} else if expectedTotalTraffic > 0 {
		t.Errorf("No traffic was reported as transmitted by ATE flows, but %d total packets were expected.", expectedTotalTraffic)
	}
}

func TestStaticGue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// configure interfaces on DUT
	configureDUT(t, dut)
	otgConfig := configureOTG(t, ate)
	enableCapture(t, ate.OTG(), otgConfig, "port2")
	enableCapture(t, ate.OTG(), otgConfig, "port1")
	enableCapture(t, ate.OTG(), otgConfig, "port3")
	ate.OTG().PushConfig(t, otgConfig)

	time.Sleep(10 * time.Second)
	configureTrafficFlows(trafficFlowData)

	type testCase struct {
		Name        string
		Description string
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config)
	}

	testCases := []testCase{
		{
			Name:        "Testcase: Validate the basic config",
			Description: "Validate traffic with basic config",
			testFunc:    testBaselineTraffic,
		},
		{
			Name:        "Testcase: Verify BE1 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify BE1 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testBE1TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF1 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF1 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF1TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF2 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF2 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF2TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF3 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF3 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF3TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF4 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF4 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF4TrafficMigration,
		},
		// {
		// 	Name:        "Testcase: DUT as a GUE Decap Node",
		// 	Description: "Verify DUT as a GUE Decap Node",
		// 	testFunc:    testDUTDecapNode,
		// },
		{
			Name:        "Testcase: Negative Scenario - EBGP Route for remote tunnel endpoints Removed",
			Description: "Verify EBGP Route for remote tunnel endpoints Removed",
			testFunc:    testTunnelEndpointRemoved,
		},
		// {
		// 	Name:        "Testcase: Negative Scenario - IBGP Route for Remote Tunnel Endpoints Removed",
		// 	Description: "Verify IBGP Route for Remote Tunnel Endpoints Removed",
		// 	testFunc:    testIbgpTunnelEndpointRemoved,
		// },
		{
			Name:        "Testcase: Establish IBGP Peering over EBGP",
			Description: "Verify Establish IBGP Peering over EBGP",
			testFunc:    testEstablishIBGPoverEBGP,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Description: %s", tc.Description)
			tc.testFunc(t, dut, ate, otgConfig)
		})
	}

}

func testBaselineTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	// Getting tunnel endpoint for flowset1, 2 and 5
	flowsets := []string{"flowSet1", "flowSet2", "flowSet5"}
	dstEndpoint := []string{}

	for _, flowset := range flowsets {
		ep := flowGroups[flowset].Endpoint
		if ep != "" {
			dstEndpoint = append(dstEndpoint, ep)
		}
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))

	// Configure GUE Encap
	cfgplugins.ConfigureGueTunnel(t, dut, "V4Udp", guePolicyName, nexthopGroupName, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)
	cfgplugins.ConfigureGueTunnel(t, dut, "V6Udp", guePolicyName, nexthopGroupNameV6, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)

	// Apply traffic policy on interface
	interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
		InterfaceID:       dut.Port(t, "port1").Name(),
		AppliedPolicyName: guePolicyName,
	}
	cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), guePolicyName, ni, interfacePolicyParams)

	cfgplugins.ConfigureUdpEncapHeader(t, dut, "ipv4-over-udp", fmt.Sprintf("%d", udpEncapPort))
	cfgplugins.ConfigureUdpEncapHeader(t, dut, "ipv6-over-udp", fmt.Sprintf("%d", udpEncapPort))

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	withdrawBGPRoutes(t, []string{ateCPort3RoutesV6})
	time.Sleep(10 * time.Second)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	withdrawBGPRoutes(t, []string{ateCPort2Routes1V4, ateCPort2Routes2V4, ateCPort2Routes1V6, ateCPort2Routes2V6})
	time.Sleep(15 * time.Second)

	// Validating no prefixes are exchanged over the IBGP peering between $ATE2_C.IBGP.v6 and $DUT_lo0.v6
	validatePrefixes(t, dut, atePort2.IPv6, false, 0, 0)

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	var flowNames []string
	for _, f := range otgConfig.Flows().Items() {
		flowNames = append(flowNames, f.Name())
	}
	validateTrafficLoss(t, ate.OTG(), flowNames)
}

func testBE1TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	flowsets := []string{"flowSet1", "flowSet5"}

	otgConfig.Flows().Clear()

	for _, flow := range flowsets {
		otgConfig.Flows().Append(flowGroups[flow].Flows[0:2]...)
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	advertiseBGPRoutes(t, []string{ateCPort2Routes1V4, ateCPort2Routes1V6, ateCPort2Routes2V4, ateCPort2Routes2V6})
	time.Sleep(15 * time.Second)

	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, atePort2.IPv6, false, 5, 0)

	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
	validateTrafficLoss(t, ate.OTG(), []string{"flowSet1-v4-1", "flowSet1-v6-1", "flowSet5-v4-1", "flowSet5-v6-1"})

	// Above flows should be GUE encapsulated
	capture := processCapture(t, ate.OTG(), "port2")
	validatePackets(t, capture, "ipv4", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
	validatePackets(t, capture, "ipv6", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)

	// Validate the counters received on ATE and DUT are same
	validateOutCounters(t, dut, ate.OTG())
}

func testAF1TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	flowsets := []string{"flowSet1", "flowSet5"}

	otgConfig.Flows().Clear()

	for _, flow := range flowsets {
		otgConfig.Flows().Append(flowGroups[flow].Flows[2:4]...)
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
	validateTrafficLoss(t, ate.OTG(), []string{"flowSet1-v4-2", "flowSet1-v6-2", "flowSet5-v4-2", "flowSet5-v6-2"})

	// Above flows should be GUE encapsulated
	capture := processCapture(t, ate.OTG(), "port2")
	validatePackets(t, capture, "ipv4", uint8(dscpValue["AF1"]), uint8(dscpValue["AF1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
	validatePackets(t, capture, "ipv6", uint8(dscpValue["AF1"]), uint8(dscpValue["AF1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
}

func testAF2TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet1"].Flows[4:]...)
	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[4:6]...)

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
	validateTrafficLoss(t, ate.OTG(), []string{"flowSet1-v4-3", "flowSet1-v6-3", "flowSet5-v4-3", "flowSet5-v6-3"})

	// Above flows should be GUE encapsulated
	capture := processCapture(t, ate.OTG(), "port2")
	validatePackets(t, capture, "ipv4", uint8(dscpValue["AF2"]), uint8(dscpValue["AF2"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
	validatePackets(t, capture, "ipv6", uint8(dscpValue["AF2"]), uint8(dscpValue["AF2"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
}

func testAF3TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {

	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet2"].Flows[0:2]...)
	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[6:8]...)

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
	validateTrafficLoss(t, ate.OTG(), []string{"flowSet2-v4-1", "flowSet2-v6-1", "flowSet5-v4-4", "flowSet5-v6-4"})

	// Above flows should be GUE encapsulated
	capture := processCapture(t, ate.OTG(), "port2")
	validatePackets(t, capture, "ipv4", uint8(dscpValue["AF3"]), uint8(dscpValue["AF3"]), nhgTTL, flowGroups["flowSet2"].Endpoint, true)
	validatePackets(t, capture, "ipv6", uint8(dscpValue["AF3"]), uint8(dscpValue["AF3"]), nhgTTL, flowGroups["flowSet2"].Endpoint, true)
}

func testAF4TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {

	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet2"].Flows[2:]...)
	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[8:10]...)

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
	validateTrafficLoss(t, ate.OTG(), []string{"flowSet2-v4-2", "flowSet2-v6-2", "flowSet5-v4-5", "flowSet5-v6-5"})

	// Above flows should be GUE encapsulated
	capture := processCapture(t, ate.OTG(), "port2")
	validatePackets(t, capture, "ipv4", uint8(dscpValue["AF4"]), uint8(dscpValue["AF4"]), nhgTTL, flowGroups["flowSet2"].Endpoint, true)
	validatePackets(t, capture, "ipv6", uint8(dscpValue["AF4"]), uint8(dscpValue["AF4"]), nhgTTL, flowGroups["flowSet2"].Endpoint, true)
}

// TODO:
// func testDUTDecapNode(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
// 	// Active flows for Flow-Set #1 through Flow-Set #4.
// 	flowsets := []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"}
// 	dstEndpoint := []string{}

// 	otgConfig.Flows().Clear()

// 	for _, flowset := range flowsets {
// 		ep := flowGroups[flowset].Endpoint
// 		if ep != "" {
// 			dstEndpoint = append(dstEndpoint, ep)
// 		}
// 		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
// 	}

// 	// Configure GUE Encap
// 	cfgplugins.ConfigureGueTunnel(t, dut, "V4Udp", guePolicyName, nexthopGroupName, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)
// 	cfgplugins.ConfigureGueTunnel(t, dut, "V6Udp", guePolicyName, nexthopGroupNameV6, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)

// 	ate.OTG().PushConfig(t, otgConfig)
// 	ate.OTG().StartProtocols(t)

// 	t.Logf("Verify OTG BGP sessions up")
// 	cfgplugins.VerifyOTGBGPEstablished(t, ate)

// 	t.Logf("Verify DUT BGP sessions up")
// 	cfgplugins.VerifyDUTBGPEstablished(t, dut)

// 	t.Logf("Starting capture")
// 	cs := startCapture(t, ate.OTG())

// 	ate.OTG().StartTraffic(t)
// 	time.Sleep(sleepTime)
// 	ate.OTG().StopTraffic(t)

// 	t.Logf("Stop Capture")
// 	stopCapture(t, ate.OTG(), cs)

// 	// Verify Traffic
// 	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
// 	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

// 	// Flows: flowSet1-v4-1 , flowSet1-v6-1 must be 100% successful
// 	validateTrafficLoss(t, ate.OTG(), []string{"flowSet1-v4-1", "flowSet1-v6-1"})

// 	// Above flows should be GUE encapsulated
// 	capture := processCapture(t, ate.OTG(), "port2")
// 	validatePackets(t, capture, "ipv4", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)
// 	validatePackets(t, capture, "ipv6", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet1"].Endpoint, true)

// 	capturePort1 := processCapture(t, ate.OTG(), "port1")
// 	validatePackets(t, capturePort1, "ipv4", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet3"].Endpoint, true)
// 	validatePackets(t, capturePort1, "ipv4", uint8(dscpValue["AF3"]), uint8(dscpValue["AF3"]), nhgTTL, flowGroups["flowSet4"].Endpoint, true)
// }

func testTunnelEndpointRemoved(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	// Active flows for Flow-Set #1 through Flow-Set #4.
	flowsets := []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"}
	dstEndpoint := []string{}

	otgConfig.Flows().Clear()

	for _, flowset := range flowsets {
		ep := flowGroups[flowset].Endpoint
		if ep != "" {
			dstEndpoint = append(dstEndpoint, ep)
		}
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	// Configure GUE Encap
	cfgplugins.ConfigureGueTunnel(t, dut, "V4Udp", guePolicyName, nexthopGroupName, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)
	cfgplugins.ConfigureGueTunnel(t, dut, "V6Udp", guePolicyName, nexthopGroupNameV6, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	withdrawBGPRoutes(t, []string{"ebgp4-te10-routes", "ebgp4-te11-routes"})
	time.Sleep(20 * time.Second)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	validatePrefixes(t, dut, atePort2.IPv6, true, 5, 0)

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	var flowNames []string
	for _, f := range otgConfig.Flows().Items() {
		flowNames = append(flowNames, f.Name())
	}
	validateTrafficLoss(t, ate.OTG(), flowNames)
}

// func testIbgpTunnelEndpointRemoved(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
// 	// Active flows for Flow-Set #1 through Flow-Set #4.
// 	flowsets := []string{"flowSet1", "flowSet2"}
// 	dstEndpoint := []string{}

// 	otgConfig.Flows().Clear()

// 	for _, flowset := range flowsets {
// 		ep := flowGroups[flowset].Endpoint
// 		if ep != "" {
// 			dstEndpoint = append(dstEndpoint, ep)
// 		}
// 		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
// 	}

// 	ate.OTG().PushConfig(t, otgConfig)
// 	ate.OTG().StartProtocols(t)

// 	t.Log("Stop advertising tunnel endpoints on ATE Port2")
// 	withdrawBGPRoutes(t, []string{atePort2RoutesTE10, atePort2RoutesTE11})
// 	time.Sleep(10 * time.Second)

// 	expectedRib := []BGPRib{
// 		{prefix: bgpInternalTE10.IPv4 + "/32", origin: atePort2.IPv4, pathId: 0, isPresent: false},
// 		{prefix: bgpInternalTE11.IPv4 + "/32", origin: atePort2.IPv4, pathId: 0, isPresent: false},
// 		{prefix: bgpInternalTE10.IPv4 + "/30", origin: atePort3.IPv4, pathId: 0, isPresent: false},
// 		{prefix: bgpInternalTE11.IPv4 + "/30", origin: atePort3.IPv4, pathId: 0, isPresent: false},
// 	}

// 	// validateBGPRib(t, dut, true, expectedRib)

// 	// TODO:
// 	// validateAFTCounters(t, dut, false, ate2ppnh1Prefix)
// 	// validateAFTCounters(t, dut, false, ate2ppnh2Prefix)
// 	// validateAFTCounters(t, dut, false, ate2InternalPrefixesV6+"/64")

// 	var flowNames []string
// 	for _, f := range otgConfig.Flows().Items() {
// 		flowNames = append(flowNames, f.Name())
// 	}
// 	validateTrafficLoss(t, ate.OTG(), flowNames)

// }

func testEstablishIBGPoverEBGP(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	// Active flows for Flow-Set #1 through Flow-Set #4.

	newTrafficData := []*trafficFlow{
		{
			name:           "flowSet6-v6-1",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{ateCPort3RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[0]},
			dstAddr:        []string{ate2InternalPrefixesV6List[0]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["BE1"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet6-v6-2",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{ateCPort3RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[1]},
			dstAddr:        []string{ate2InternalPrefixesV6List[1]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF1"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet6-v6-3",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{ateCPort3RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[2]},
			dstAddr:        []string{ate2InternalPrefixesV6List[2]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF2"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE11.IPv4,
		},
		{
			name:           "flowSet7-v6-1",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{ateCPort3RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[3]},
			dstAddr:        []string{ate2InternalPrefixesV6List[3]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF3"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
		{
			name:           "flowSet7-v6-2",
			srcDevice:      []string{atePort1RouteV6},
			dstDevice:      []string{ateCPort3RoutesV6},
			srcAddr:        []string{ate1UserPrefixesV6List[4]},
			dstAddr:        []string{ate2InternalPrefixesV6List[4]},
			trafficPps:     trafficPps,
			packetSize:     totalPackets,
			dscp:           uint8(dscpValue["AF4"]),
			v4Traffic:      false,
			tunnelEndpoint: bgpInternalTE10.IPv4,
		},
	}

	otgConfig.Flows().Clear()
	configureTrafficFlows(newTrafficData)

	flowsets := []string{"flowSet6", "flowSet7", "flowSet3", "flowSet4"}
	dstEndpoint := []string{}

	for _, flowset := range flowsets {
		ep := flowGroups[flowset].Endpoint
		if ep != "" {
			dstEndpoint = append(dstEndpoint, ep)
		}
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	// Configure GUE Encap
	cfgplugins.ConfigureGueTunnel(t, dut, "V4Udp", guePolicyName, nexthopGroupName, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)
	cfgplugins.ConfigureGueTunnel(t, dut, "V6Udp", guePolicyName, nexthopGroupNameV6, loopbackIntfName, dstEndpoint, []string{}, nhgTTL)

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Log("Stop advertising tunnel endpoints on ATE Port2")
	withdrawBGPRoutes(t, []string{ateCPort2Routes1V6, ateCPort2Routes2V6})
	advertiseBGPRoutes(t, []string{ateCPort3RoutesV6})

	time.Sleep(20 * time.Second)

	d := &oc.Root{}
	i := d.GetOrCreateInterface(dut.Port(t, "port2").Name())
	i.SetEnabled(false)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Config(), i)

	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime)
	ate.OTG().StopTraffic(t)

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	var flowNames []string
	for _, f := range otgConfig.Flows().Items() {
		flowNames = append(flowNames, f.Name())
	}
	validateTrafficLoss(t, ate.OTG(), flowNames)
	capture := processCapture(t, ate.OTG(), "port3")
	validatePackets(t, capture, "ipv6", uint8(dscpValue["BE1"]), uint8(dscpValue["BE1"]), nhgTTL, flowGroups["flowSet6"].Endpoint, true)

}

func validateBGPRib(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, expectedPfx []BGPRib) {
	found := 0

	bgpRIBPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
	if isV4 {
		locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.
			AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
		for route, prefix := range locRib.Route {
			for _, expectedRoute := range expectedPfx {
				if prefix.GetPrefix() == expectedRoute.prefix {
					if !expectedRoute.isPresent {
						t.Errorf("Not expected: Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
						break
					}
					found++
				}
			}
		}
		if found != len(expectedPfx) {
			t.Fatalf("Not all V4 routes found. expected:%d got:%d", len(expectedPfx), found)
		}

	} else {
		locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPath.
			AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
		for route, prefix := range locRib.Route {
			for _, expectedRoute := range expectedPfx {
				if prefix.GetPrefix() == expectedRoute.prefix {
					if !expectedRoute.isPresent {
						t.Errorf("Not expected: Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
						break
					}
					found++
				}
			}
		}
		if found != len(expectedPfx) {
			t.Fatalf("Not all V6 routes found. expected:%d got:%d", len(expectedPfx), found)
		}
	}
}

func validateAFTCounters(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, routeIp string) {
	t.Logf("Validate AFT parameters for %s", routeIp)
	if isV4 {
		ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(routeIp)
		if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			ipv4Entry, present := val.Val()
			return present && ipv4Entry.GetPrefix() == routeIp
		}).Await(t); !ok {
			t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, routeIp)
		}
	} else {
		ipv6Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(routeIp)
		if got, ok := gnmi.Watch(t, dut, ipv6Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			ipv6Entry, present := val.Val()
			return present && ipv6Entry.GetPrefix() == routeIp
		}).Await(t); !ok {
			t.Errorf("ipv6-entry/state/prefix got %v, want %s", got, routeIp)
		}
	}
}
