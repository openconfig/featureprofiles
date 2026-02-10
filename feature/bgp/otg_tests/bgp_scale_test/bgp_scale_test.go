// Copyright 2025 Google LLC
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

package bgp_scale_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	netinstbgp "github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4        = uint8(30)
	plenIPv6        = uint8(126)
	v4VlanPlen      = uint8(24)
	v6VlanPlen      = uint8(112)
	dutAS           = 64500
	peervRR4GrpName = "BGP-RR-GROUP-V4"
	peervRR6GrpName = "BGP-RR-GROUP-V6"
	peervVF4GrpName = "BGP-VF-GROUP-V4"
	peervVF6GrpName = "BGP-VF-GROUP-V6"
	peerv41GrpName  = "BGP-PEER-GROUP-V41-"
	peerv61GrpName  = "BGP-PEER-GROUP-V61-"
	peerv42GrpName  = "BGP-PEER-GROUP-V42-"
	peerv62GrpName  = "BGP-PEER-GROUP-V62-"
	peerv43GrpName  = "BGP-PEER-GROUP-V43-"
	peerv63GrpName  = "BGP-PEER-GROUP-V63-"
	peerv44GrpName  = "BGP-PEER-GROUP-V44-"
	peerv64GrpName  = "BGP-PEER-GROUP-V64-"
	dutP2IPv4       = "102.1.1.1"
	dutP2IPv6       = "1002::102:1:1:1"
	dutP3IPv4       = "153.1.1.1"
	dutP3IPv6       = "1003::153:1:1:1"
	dutP4IPv4       = "200.1.1.1"
	dutP4IPv6       = "1004::200:1:1:1"
	ateP2IPv4       = "102.1.1.2"
	ateP2IPv6       = "1002::102:1:1:2"
	ateP3IPv4       = "153.1.1.2"
	ateP3IPv6       = "1003::153:1:1:2"
	ateP4IPv4       = "200.1.1.2"
	ateP4IPv6       = "1004::200:1:1:254"
	ateP4LoopbackV4 = "203.0.113.0"
	ateP4LoopbackV6 = "2001:db8::203:0:113:0"
	dutP4LoopbackV4 = "203.0.113.200"
	dutP4LoopbackV6 = "2001:db8::203:0:113:200"
	ateP4V4Route    = "60.0.0.1"
	// ISIS related constants
	isisInstance       = "DEFAULT"
	dutAreaAddress     = "49.0001"
	ateAreaAddress     = "49"
	dutSysID           = "1920.0000.2001"
	otgIsisPort4LoopV4 = "203.0.113.14"
	otgIsisPort4LoopV6 = "2001:db8::203:0:113:14"
	trafficDuration    = 30 * time.Second
	tolerancePct       = 2
	lagType            = oc.IfAggregate_AggregationType_LACP
	juniperLimitation  = "routing-options {\nmaximum-ecmp 64;\n}\nprotocols {\nlldp {\nport-id-subtype interface-name;\n}\nbgp {\npreference 9;\n}\n}\npolicy-options {\npolicy-statement balance {\nthen load-balance per-packet;\n}\n}"
	// BGP related constants
	defaultHoldTimer      = 240
	defaultKeepaliveTimer = 80
	defaultUpdateTimer    = 30
	defaultEnablePGGR     = true
)

var (
	defaultNetworkInstance        = ""
	dutIntfs                      = []string{}
	start                         = time.Now()
	delLinkbwCLIConfig            = ""
	isSetupPhase                  = false
	isMSSTest                     = false
	v4IBGPRouteCount       uint32 = 100
	v6IBGPRouteCount       uint32 = 100
	ipv4Traffic                   = map[int][]string{}
	ipv6Traffic                   = map[int][]string{}
	aggIDs                 []string
	aggs                   []*oc.Interface
	distanceConfigv4       = ""
	distanceConfigv6       = ""
	weightNHConfig         = ""
	kneDeviceModelList     = []string{"ncptx", "ceos", "srlinux", "xrd", "8000e"}
	lb                     string
	dutPort1               = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			Desc:    "port1",
			IPv4:    "192.0.2.1",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:1",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 1,
		index:      0,
		ip4: func(vlan int) (string, string) {
			return "192.0.2.1", ""
		},
		ip6: func(vlan int) (string, string) {
			return "2001:db8::192:0:2:1", ""
		},
		pg4:          peerv41GrpName,
		pg6:          peerv61GrpName,
		asn:          ateAS4Byte,
		pgList:       []string{peerv41GrpName, peerv61GrpName},
		importPolicy: "CONVERGENCE-IN",
		exportPolicy: "CONVERGENCE-OUT",
	}
	dutPort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			Desc:    "port2",
			IPv4:    "102.1.1.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1002::102:1:1:1",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf:   10,
		index:        1,
		ip4:          dutPort2IPv4,
		ip6:          dutPort2IPv6,
		pg4:          peerv42GrpName,
		pg6:          peerv62GrpName,
		pgList:       []string{peerv42GrpName, peerv62GrpName},
		importPolicy: "ALLOW-IN",
		exportPolicy: "ALLOW-OUT",
	}
	dutPort3 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port3",
			Desc:    "port3",
			IPv4:    "153.1.1.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1003::153:1:1:1",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf:   10,
		index:        2,
		ip6:          dutPort3IPv6,
		ip4:          dutPort3IPv4,
		pg4:          peerv43GrpName,
		pg6:          peerv63GrpName,
		pgList:       []string{peerv43GrpName, peerv63GrpName},
		importPolicy: "ALLOW-IN",
		exportPolicy: "ALLOW-OUT",
	}
	dutPort4 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port4",
			Desc:    "port4",
			IPv4:    "200.1.1.1",
			IPv4Len: 30,
			IPv6:    "1004::200:1:1:1",
			IPv6Len: 126,
		},
		numSubIntf:   7,
		index:        3,
		ip4:          dutPort4IPv4,
		ip6:          dutPort4IPv6,
		ip4Loopback:  dutPort4v4Loopback,
		ip6Loopback:  dutPort4v6Loopback,
		pg4:          peerv44GrpName,
		pg6:          peerv64GrpName,
		pgList:       []string{peervRR4GrpName, peervRR6GrpName, peervVF4GrpName, peervVF6GrpName},
		importPolicy: "IBGP-IN",
		exportPolicy: "IBGP-OUT",
	}
	atePort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "192.0.2.2",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:2",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 1,
		index:      0,
		ip4: func(vlan int) (string, string) {
			return "192.0.2.2", ""
		},
		ip6: func(vlan int) (string, string) {
			return "2001:db8::192:0:2:2", ""
		},
		gateway: func(vlan int) (string, string) {
			return "192.0.2.1", ""
		},
		gateway6: func(vlan int) (string, string) {
			return "2001:db8::192:0:2:1", ""
		},
		pg4:          peerv41GrpName,
		pg6:          peerv61GrpName,
		pgList:       []string{peerv41GrpName, peerv61GrpName},
		importPolicy: "CONVERGENCE-IN",
		exportPolicy: "CONVERGENCE-OUT",
	}
	atePort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "102.1.1.2",
			IPv4Len: v4VlanPlen,
			IPv6:    "1002::102:1:1:2",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf:   10,
		index:        1,
		ip4:          atePort2IPv4,
		ip6:          atePort2IPv6,
		gateway:      dutPort2IPv4,
		gateway6:     dutPort2IPv6,
		pg4:          peerv42GrpName,
		pg6:          peerv62GrpName,
		pgList:       []string{peerv42GrpName, peerv62GrpName},
		importPolicy: "ALLOW-IN",
		exportPolicy: "ALLOW-OUT",
	}
	atePort3 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port3",
			MAC:     "02:00:03:01:01:01",
			IPv4:    "153.1.1.2",
			IPv4Len: v4VlanPlen,
			IPv6:    "1003::153:1:1:2",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf:   10,
		index:        2,
		ip6:          atePort3IPv6,
		ip4:          atePort3IPv4,
		gateway:      dutPort3IPv4,
		gateway6:     dutPort3IPv6,
		pg4:          peerv43GrpName,
		pg6:          peerv63GrpName,
		pgList:       []string{peerv43GrpName, peerv63GrpName},
		importPolicy: "ALLOW-IN",
		exportPolicy: "ALLOW-OUT",
	}
	atePort4 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port4",
			MAC:     "02:00:04:01:01:01",
			IPv4:    "200.1.1.2",
			IPv4Len: 30,
			IPv6:    "1004::200:1:1:254",
			IPv6Len: 126,
		},
		ateISISSysID:     "64000000000",
		v4Route:          atePort4v4Route,
		v4ISISRouteCount: 100,
		v6Route:          atePort4v6Route,
		v6ISISRouteCount: 100,
		numSubIntf:       7,
		index:            3,
		ip4:              atePort4IPv4,
		ip6:              atePort4IPv6,
		gateway:          dutPort4IPv4,
		gateway6:         dutPort4IPv6,
		ip4Loopback:      atePort4v4Loopback,
		ip6Loopback:      atePort4v6Loopback,
		lagMAC:           "02:55:10:10:10:01",
		ethMAC:           "02:55:10:10:10:02",
		port1MAC:         "02:55:10:10:10:03",
		pg4:              peerv44GrpName,
		pg6:              peerv64GrpName,
		pgList:           []string{peervRR4GrpName, peervRR6GrpName, peervVF4GrpName, peervVF6GrpName},
		importPolicy:     "IBGP-IN",
		exportPolicy:     "IBGP-OUT",
	}
	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.1.113.200",
		IPv6:    "2001:1:db8::203:0:113:200",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	dutagg24 = attrs.Attributes{
		Desc:    "DUT to ATE LAG4",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}
)

type bgpGroupConfig struct {
	pgName           string
	pgAS             uint32
	pgHoldTimer      uint16
	pgKeepaliveTimer uint16
	pgUpdateTimer    uint16
	enablePGGR       bool
	pgV4Unicast      bool
	pgV6Unicast      bool
	pgAFISAFI        oc.E_BgpTypes_AFI_SAFI_TYPE
	pgImportPolicy   string
	pgExportPolicy   string
	pgTransport      string
	pgMultipath      bool
}

type dutData struct {
	isisData *cfgplugins.ISISGlobalParams
	lags     []*cfgplugins.DUTAggData
}

var dut2Data = dutData{
	isisData: &cfgplugins.ISISGlobalParams{
		DUTArea:  "49.0001",
		DUTSysID: "1920.0000.2001",
	},
	lags: []*cfgplugins.DUTAggData{
		{
			Attributes:      dutagg24,
			SubInterfaces:   []*cfgplugins.DUTSubInterfaceData{},
			OndatraPortsIdx: []int{3},
			LacpParams:      lacpParams,
			AggType:         lagType,
		},
	},
}

type attributes struct {
	*attrs.Attributes
	numSubIntf       uint32
	index            uint8
	ateISISSysID     string
	v4Route          func(vlan int) string
	v4ISISRouteCount uint32
	v6Route          func(vlan int) string
	v6ISISRouteCount uint32
	ip4              func(vlan int) (string, string)
	ip6              func(vlan int) (string, string)
	gateway          func(vlan int) (string, string)
	gateway6         func(vlan int) (string, string)
	ip4Loopback      func(vlan int) (string, string)
	ip6Loopback      func(vlan int) (string, string)
	lagMAC           string
	ethMAC           string
	port1MAC         string
	pg4              string
	pg6              string
	asn              func(vlan int) uint32
	pgList           []string
	importPolicy     string // Import policy of the neighbor
	exportPolicy     string // Export policy of the neighbor
}

type dutInfo struct {
	processes map[string]processesInfo
}

type processesInfo struct {
	pid    uint64
	cpuPct uint8
	memPct uint8
}

type bgpNeighbor struct {
	as           uint32 // AS number of the neighbor
	neighborip   string // IP address of the neighbor
	isV4         bool   // True if the neighbor is IPv4, false if IPv6
	pg           string // Peer group of the neighbor
	importPolicy string
	exportPolicy string
}

// dutPort2IPv4 returns ipv4 addresses for every vlanID.
func dutPort2IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP2IPv4, vlan*256)
	return ip, err
}

// dutPort2IPv6 returns ip6 addresses for every vlanID.
func dutPort2IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP2IPv6, vlan*256*256)
	return ip, err
}

// atePort2IPv4 returns ip4 addresses for every vlanID.
func atePort2IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP2IPv4, vlan*256)
	return ip, err
}

// atePort2IPv6 returns ip6 addresses for every vlanID.
func atePort2IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP2IPv6, vlan*256*256)
	return ip, err
}

// dutPort3IPv4 returns ipv4 addresses for every vlanID.
func dutPort3IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP3IPv4, vlan*256)
	return ip, err
}

// dutPort3IPv6 returns ip6 addresses for every vlanID.
func dutPort3IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP3IPv6, vlan*256*256)
	return ip, err
}

// atePort3IPv4 returns ip4 addresses for every vlanID.
func atePort3IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP3IPv4, vlan*256)
	return ip, err
}

// atePort3IPv6 returns ip6 addresses for every vlanID.
func atePort3IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP3IPv6, vlan*256*256)
	return ip, err
}

// dutPort4IPv4 returns ipv4 addresses for every vlanID.
func dutPort4IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP4IPv4, vlan*256)
	return ip, err
}

// dutPort4IPv6 returns ip6 addresses for every vlanID.
func dutPort4IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP4IPv6, vlan*256*256)
	return ip, err
}

// atePort4IPv4 returns ip4 addresses for every vlanID.
func atePort4IPv4(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP4IPv4, vlan*256)
	return ip, err
}

// atePort4IPv6 returns ip6 addresses for every vlanID.
func atePort4IPv6(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP4IPv6, vlan*256*256)
	return ip, err
}

// atePort4v4Route returns ip addresses starting 60.%d.0.1 for every vlanID.
func atePort4v4Route(vlan int) string {
	ip, _ := cfgplugins.IncrementIP(ateP4V4Route, vlan*256*256)
	return ip
}

// atePort4v4Loopback returns ip addresses starting 203.0.113.%d for every vlanID.
func atePort4v4Loopback(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP4LoopbackV4, vlan)
	return ip, err
}

// atePort4v6Loopback returns ip addresses starting 2001:db8::203:0:113:%d for every vlanID.
func atePort4v6Loopback(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP4LoopbackV6, vlan)
	return ip, err
}

// dutPort4v4Loopback returns ip addresses starting 203.0.113.%d for every vlanID.
func dutPort4v4Loopback(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP4LoopbackV4, vlan)
	return ip, err
}

// dutPort4v6Loopback returns ip addresses starting 2001:db8::203:0:113:%d for every vlanID.
func dutPort4v6Loopback(vlan int) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP4LoopbackV6, vlan)
	return ip, err
}

// atePort4v6Route returns ip addresses starting 2010:%d:db8:64:64::1 for every vlanID.
func atePort4v6Route(vlan int) string {
	return fmt.Sprintf("2010:%d:db8:64:64::1", vlan)
}

// ateAS4Byte returns the AS number for every vlanID.
func ateAS4Byte(vlan int) uint32 {
	return 800000 + uint32(vlan)*100
}

func vlanID(i int) int {
	return int(dutPort4.index)*10 + i
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBGPScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	top := gosnappi.NewConfig()

	ate.Topology().New()

	dutInfoInitial := dutInfo{
		processes: make(map[string]processesInfo),
	}
	t.Logf("Verify Initial system health")
	dutInfoInitial = verifySystemHealth(t, dut)

	nbrList := setupNeighborList([]attributes{atePort1, atePort2, atePort3, atePort4})

	testCases := []struct {
		name              string
		scale             map[string]uint32
		dutSetup          func(t *testing.T, dut *ondatra.DUTDevice, top gosnappi.Config, ate *ondatra.ATEDevice, nbrList []*bgpNeighbor, operation string, isMSSTest bool)
		validate          []func(t *testing.T, dev *ondatra.Device)
		bgpEvents         func(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, nbrList []*bgpNeighbor, dutInfoPreReset dutInfo, operation string, v4Convergence, v6Convergence uint32)
		operation         string
		verifyTelemetry   []func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor)
		verifyConvergence func(t *testing.T, dut *ondatra.DUTDevice, afi []string, v4RouteCount, v6RouteCount uint32, isRouteWithdrawn bool)
		verifyTraffic     []func(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config)
		testCleanup       []func(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16, nbrList []string, mss uint16)
	}{
		{
			name:              "RT-1.65.1.1 - Steady State - 7 v4/v6 IBGP sessions",
			dutSetup:          dutSetup,
			scale:             map[string]uint32{"Port2": 200, "Port3": 200, "Port4": 7, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 6000, "v6IBGPRoutes": 6000, "v4Convergence": 6100, "v6Convergence": 6005},
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   []func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor){verifyISISTelemetry, verifyBgpTelemetry, verifyBGPCapabilities, validateLinkBWNotAdvertised},
			verifyConvergence: measureConvergence,
			verifyTraffic:     []func(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config){sendTraffic, verifyTraffic},
		},
		{
			name:              "RT-1.65.2 - BGP Session Scale - 31 v4/v6 IBGP sessions",
			dutSetup:          ateSetup,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 6000, "v6IBGPRoutes": 6000, "v4Convergence": 6100, "v6Convergence": 6005},
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   []func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor){verifyISISTelemetry, verifyBgpTelemetry, verifyBGPCapabilities},
			verifyConvergence: measureConvergence,
			verifyTraffic:     []func(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config){sendTraffic, verifyTraffic},
		},
		{
			name:              "RT-1.65.3 - Session and BGP Route Scale without BGP events",
			dutSetup:          ateSetup,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 250000, "v6IBGPRoutes": 150000, "v4Convergence": 250100, "v6Convergence": 150005},
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   []func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor){verifyISISTelemetry, verifyBgpTelemetry, verifyBGPCapabilities},
			verifyConvergence: measureConvergence,
			verifyTraffic:     []func(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config){sendTraffic, verifyTraffic},
		},
		{
			name:              "RT-1.65.3.2 - Session and BGP Route Scale without BGP events with MSS configured",
			dutSetup:          ateSetup,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 250000, "v6IBGPRoutes": 150000, "v4Convergence": 250100, "v6Convergence": 150005},
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   []func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor){verifyISISTelemetry, verifyBgpTelemetry, verifyBGPCapabilities},
			verifyConvergence: measureConvergence,
			verifyTraffic:     []func(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config){sendTraffic, verifyTraffic},
			testCleanup:       []func(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16, nbrList []string, mss uint16){cfgplugins.DeleteDUTBGPMaxSegmentSize},
		},
		{
			name:              "RT-1.65.4.1 - BGP Session and Route Scale with BGP events_Alter_BGP_attributes",
			dutSetup:          nil,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 250000, "v6IBGPRoutes": 150000, "v4Convergence": 250100, "v6Convergence": 150005},
			bgpEvents:         bgpEvents,
			operation:         "alterBGPAttributes",
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   nil,
			verifyConvergence: nil,
			verifyTraffic:     nil,
		},
		{
			name:              "RT-1.65.4.2 - BGP Session and Route Scale with BGP events_WithdrawRoutes",
			dutSetup:          nil,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 250000, "v6IBGPRoutes": 150000, "v4Convergence": 250100, "v6Convergence": 150005},
			bgpEvents:         bgpEvents,
			operation:         "withdrawRoutes",
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   nil,
			verifyConvergence: nil,
			verifyTraffic:     nil,
		},
		{
			name:              "RT-1.65.4.3 - BGP Session and Route Scale with BGP events_ResetBGPPeers",
			dutSetup:          nil,
			scale:             map[string]uint32{"Port2": 400, "Port3": 400, "Port4": 31, "v4ISISRoutes": 1000, "v6ISISRoutes": 1000, "v4IBGPRoutes": 250000, "v6IBGPRoutes": 150000, "v4Convergence": 250100, "v6Convergence": 150005},
			bgpEvents:         bgpEvents,
			operation:         "resetBGPPeers",
			validate:          []func(t *testing.T, dev *ondatra.Device){verifyPortsUp},
			verifyTelemetry:   nil,
			verifyConvergence: nil,
			verifyTraffic:     nil,
		},
	}
	for index, tc := range testCases {
		if tc.dutSetup != nil {
			isSetupPhase = true
			t.Run(tc.name+" DUT Setup", func(t *testing.T) {
				t.Logf("BGP event: %v", tc.name)
				t.Logf("BGP scale: Prepare scale")
				atePort2.numSubIntf = tc.scale["Port2"]
				atePort3.numSubIntf = tc.scale["Port3"]
				atePort4.numSubIntf = tc.scale["Port4"]
				dutPort2.numSubIntf = 400
				dutPort3.numSubIntf = 400
				dutPort4.numSubIntf = 31
				atePort4.v4ISISRouteCount = tc.scale["v4ISISRoutes"] // ISIS v4 Scale
				atePort4.v6ISISRouteCount = tc.scale["v6ISISRoutes"] // ISIS v6 Scale
				v4IBGPRouteCount = tc.scale["v4IBGPRoutes"]          // iBGP v4Scale
				v6IBGPRouteCount = tc.scale["v6IBGPRoutes"]          // iBGP v6 Scale

				ipv4Traffic = map[int][]string{}
				ipv6Traffic = map[int][]string{}
				nbrList := setupNeighborList([]attributes{atePort1, atePort2, atePort3, atePort4})

				if index > 0 {
					ate.Topology().New()
					top = gosnappi.NewConfig()
				}
				t.Logf("BGP scale: Prepare scale done %v", tc.scale)

				t.Logf("BGP scale: Prepare dutSetup ")
				if strings.Contains(tc.name, "MSS") {
					isMSSTest = true
				} else {
					isMSSTest = false
				}
				tc.dutSetup(t, dut, top, ate, nbrList, "NO_OP", isMSSTest)
			})
		} else {
			isSetupPhase = false
		}

		if tc.validate != nil {
			t.Run(tc.name+" Verify Ports Up", func(t *testing.T) {
				t.Logf("Validate Port status")

				for _, v := range tc.validate {
					v(t, dut.Device)
				}
			})
		}

		if tc.verifyConvergence != nil {
			t.Run(tc.name+" Verify BGP Convergence", func(t *testing.T) {
				t.Logf("Verify Convergence")
				tc.verifyConvergence(t, dut, []string{"ipv4", "ipv6"}, tc.scale["v4Convergence"], tc.scale["v6Convergence"], false)
			})
		}

		if tc.bgpEvents != nil {
			t.Run(tc.name+" "+tc.operation, func(t *testing.T) {
				t.Logf("BGP event: %v", tc.operation)
				tc.bgpEvents(t, ate, dut, top, nbrList, dutInfoInitial, tc.operation, tc.scale["v4Convergence"], tc.scale["v6Convergence"])
			})
		}

		if tc.verifyTelemetry != nil {
			t.Run(tc.name+" Verify ISIS/BGP/System Telemetry", func(t *testing.T) {
				t.Logf("Verify Telemetry")

				for _, v := range tc.verifyTelemetry {
					v(t, dut, ate, top, nbrList)
				}
			})
		}
		if tc.verifyTraffic != nil {
			t.Run(tc.name+" Verify Traffic", func(t *testing.T) {
				t.Logf("Verify Traffic")
				for _, v := range tc.verifyTraffic {
					v(t, ate, top)
				}
			})
		}
		if tc.testCleanup != nil {
			t.Run(tc.name+" Test Cleanup", func(t *testing.T) {
				t.Logf("Verify MSS for all ports")
				verifyMSS(t, dut, nbrList, []attributes{dutPort1, dutPort4})
				t.Logf("Verify MSS for all ports done")
				t.Logf("Test Cleanup")
				for _, v := range tc.testCleanup {
					var nbrPeers []string
					for i := 1; i <= int(dutPort4.numSubIntf); i++ {
						ip4, eMsg := atePort4v4Loopback(i)
						if eMsg != "" {
							t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
						}
						nbrPeers = append(nbrPeers, ip4)
						ip6, eMsg := atePort4v6Loopback(i)
						if eMsg != "" {
							t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
						}
						nbrPeers = append(nbrPeers, ip6)
					}
					v(t, dut, dut.Port(t, dutPort1.Name).Name(), 9210, []string{"192.0.2.2", "2001:db8::192:0:2:2"}, 4096)
					v(t, dut, aggIDs[0], 9210, nbrPeers, 4096)
				}
				isMSSTest = false
			})
		}
	}
}

// measureConvergence measures the convergence time for BGP routes.
func measureConvergence(t *testing.T, dut *ondatra.DUTDevice, afi []string, v4RouteCount, v6RouteCount uint32, isRouteWithdrawn bool) {
	t.Helper()
	if afi == nil {
		afi = []string{"ipv4", "ipv6"}
	}
	t.Logf("Measure Convergence for %v", afi)

	testConvergence := make([]func(testing.TB), len(afi))

	for i, safi := range afi {
		testConvergence[i] = func(t testing.TB) {
			time.Sleep(10 * time.Second)
			statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
				Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
			var prefixes *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_PrefixesPath
			var peer string
			var routeCount, routeCountWithdraw, prevSent uint32
			var counter int
			var convergenceTime time.Duration

			if isRouteWithdrawn && safi == "ipv4" {
				prevSent = v4RouteCount
			} else if isRouteWithdrawn && safi == "ipv6" {
				prevSent = v6RouteCount
			}
		ConvergenceLoop:
			for repeat := 1000; repeat > 0; repeat-- {
				switch safi {
				case "ipv4":
					prefixes = statePath.Neighbor(atePort1.IPv4).
						AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
					peer = atePort1.IPv4
					routeCount = v4RouteCount
					if isRouteWithdrawn {
						routeCountWithdraw = 200
					}

				case "ipv6":
					prefixes = statePath.Neighbor(atePort1.IPv6).
						AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
					peer = atePort1.IPv6
					routeCount = v6RouteCount
					if isRouteWithdrawn {
						routeCountWithdraw = 100
					}
				}
				gotSent := gnmi.Get(t, dut, prefixes.Sent().State())
				switch {
				case !isRouteWithdrawn && gotSent >= routeCount:
					t.Logf("%s Convergence: Prefixes sent from ingress port are learnt at ATE dst port : %v are %v", safi, peer, routeCount)
					t.Logf("%s Convergence: Time taken for convergence: %v", safi, time.Since(start))
					break ConvergenceLoop
				case !isRouteWithdrawn && repeat > 0 && gotSent < routeCount:
					if gotSent != prevSent {
						prevSent = gotSent
						counter = 0
						convergenceTime = 0
					} else if gotSent == prevSent && counter <= 20 && prevSent > 0 {
						if convergenceTime == 0 && counter == 0 {
							convergenceTime = time.Since(start)
						}
						counter++
						t.Logf("%s Convergence: sent prefixes from DUT to neighbor %v is is same as previous: got %v, previous %v, want %v", safi, peer, gotSent, prevSent, routeCount)
						t.Logf("%s Convergence: Sleep for 30 secs", safi)
						time.Sleep(time.Second * 30)
					} else if gotSent == prevSent && counter > 20 && prevSent > 0 {
						t.Logf("%s Convergence: Time taken for convergence: %v", safi, convergenceTime)
						t.Logf("%s Convergence without convergenceTime var: Time taken for convergence: %v", safi, time.Since(start))
						break ConvergenceLoop
					}
					t.Logf("%s Convergence: All the prefixes are not learnt , wait for 30 secs before retry.. got %v, want %v", safi, gotSent, routeCount)
					time.Sleep(time.Second * 30)
				case !isRouteWithdrawn && repeat == 0 && gotSent < routeCount:
					t.Errorf("%s Convergence: sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", safi, peer, gotSent, routeCount)
					time.Sleep(time.Second * 30)
				case isRouteWithdrawn && gotSent <= routeCountWithdraw:
					t.Logf("%s Convergence WithdrawRoutes: Prefixes sent from ingress port are learnt at ATE dst port : %v are %v", safi, peer, routeCountWithdraw)
					t.Logf("%s Convergence WithdrawRoutes: Time taken for convergence: %v", safi, time.Since(start))
					break ConvergenceLoop
				case isRouteWithdrawn && repeat > 0 && gotSent > routeCountWithdraw:
					if gotSent != prevSent {
						prevSent = gotSent
						counter = 0
						convergenceTime = 0
					} else if gotSent == prevSent && counter <= 20 {
						if convergenceTime == 0 && counter == 0 {
							convergenceTime = time.Since(start)
						}
						counter++
						t.Logf("%s Convergence WithdrawRoutes: sent prefixes from DUT to neighbor %v is is same as previous: got %v, previous %v, want %v", safi, peer, gotSent, prevSent, routeCountWithdraw)
						t.Logf("%s Convergence: Sleep for 30 secs", safi)
						time.Sleep(time.Second * 30)
					} else if gotSent == prevSent && counter > 20 {
						t.Logf("%s Convergence WithdrawRoutes: Time taken for convergence: %v", safi, convergenceTime)
						t.Logf("%s Convergence WithdrawRoutes: Convergence without convergenceTime var: Time taken for convergence: %v", safi, time.Since(start))
						break ConvergenceLoop
					}
					t.Logf("%s Convergence WithdrawRoutes: All the prefixes are not learnt , wait for 30 secs before retry.. got %v, want %v", safi, gotSent, routeCountWithdraw)
					time.Sleep(time.Second * 30)
				case isRouteWithdrawn && repeat == 0 && gotSent > routeCountWithdraw:
					t.Errorf("%s Convergence WithdrawRoutes: sent prefixes from DUT to neighbor %v is mismatch: got %v, want %v", safi, peer, gotSent, routeCountWithdraw)
					time.Sleep(time.Second * 30)
				default:
					if isRouteWithdrawn {
						t.Logf("DUT did not converge for safi %v after %v retries. got %v want %v routes", safi, repeat, gotSent, routeCountWithdraw)
					} else {
						t.Logf("DUT did not converge for safi %v after %v retries. got %v want %v routes", safi, repeat, gotSent, routeCount)
					}
				}
			}
		}
	}

	t.Logf("Starting parallel convergence tests")

	testt.ParallelFatal(t, testConvergence...)

	t.Logf("Measure Convergence completed")
}

// ateReSetup configures the ATE for the test.
func ateSetup(t *testing.T, dut *ondatra.DUTDevice, top gosnappi.Config, ate *ondatra.ATEDevice, nbrList []*bgpNeighbor, operation string, isMSSTest bool) {
	t.Helper()
	if isMSSTest {
		t.Logf("Configuring DUT for MSS test")
		var nbrPeers []string
		for i := 1; i <= int(dutPort4.numSubIntf); i++ {
			ip4, eMsg := atePort4v4Loopback(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
			}
			nbrPeers = append(nbrPeers, ip4)
			ip6, eMsg := atePort4v6Loopback(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
			}
			nbrPeers = append(nbrPeers, ip6)
		}
		cfgplugins.ConfigureDUTBGPMaxSegmentSize(t, dut, dut.Port(t, dutPort1.Name).Name(), 9210, []string{"192.0.2.2", "2001:db8::192:0:2:2"}, 4096)
		cfgplugins.ConfigureDUTBGPMaxSegmentSize(t, dut, aggIDs[0], 9210, nbrPeers, 4096)
	}
	var vlan, mtu uint32
	var isLoopback, isLag, isMTU bool
	var loopbackV4, loopbackV6, eMsg string
	for _, atePort := range []attributes{atePort1, atePort2, atePort3, atePort4} {
		if dut.Vendor() == ondatra.CISCO {
			mtu = 9214
		} else {
			mtu = 9210
		}
		if atePort.Name == "port4" {
			isLoopback = true
			isLag = true
			isMTU = isMSSTest
		} else {
			isLoopback = false
			isLag = false
			isMTU = false
			if atePort.Name == "port1" {
				isMTU = isMSSTest
				mtu = 9210
			}
		}

		intefaces := []*otgconfighelpers.InterfaceProperties{}
		for i := 1; i <= int(atePort.numSubIntf); i++ {
			mac, err := incrementMAC(atePort.MAC, int(atePort.index*10)+int(i)+1)
			if err != nil {
				t.Fatalf("incrementMAC(%v, %v) failed: %v", atePort.MAC, int(atePort.index*10)+int(i)+1, err)
			}
			if atePort.Name == "port1" {
				vlan = 0
				isMTU = isMSSTest
			} else {
				vlan = uint32(int(atePort.index*10) + i)
			}
			if isLoopback {
				loopbackV4, eMsg = atePort.ip4Loopback(i)
				if eMsg != "" {
					t.Fatalf("Failed to generate IPv4 loopback address with error '%s'", eMsg)
				}
				loopbackV6, eMsg = atePort.ip6Loopback(i)
				if eMsg != "" {
					t.Fatalf("Failed to generate IPv6 loopback address with error '%s'", eMsg)
				}
			} else {
				loopbackV4 = ""
				loopbackV6 = ""
			}
			ip4, eMsg := atePort.ip4(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
			}
			ip6, eMsg := atePort.ip6(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
			}
			gw4, eMsg := atePort.gateway(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv4 gateway with error '%s'", eMsg)
			}
			gw6, eMsg := atePort.gateway6(i)
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv6 gateway with error '%s'", eMsg)
			}
			intefaces = append(intefaces, &otgconfighelpers.InterfaceProperties{
				Name:        fmt.Sprintf(`%sdst%d`, atePort.Name, i),
				IPv4:        ip4,
				IPv4Gateway: gw4,
				IPv4Len:     30,
				Vlan:        vlan,
				MAC:         mac,
				IPv6:        ip6,
				IPv6Gateway: gw6,
				IPv6Len:     126,
				LoopbackV4:  loopbackV4,
				LoopbackV6:  loopbackV6,
			})
		}
		port := otgconfighelpers.Port{
			Name:        atePort.Name,
			AggMAC:      atePort.MAC,
			Interfaces:  intefaces,
			IsLag:       isLag,
			IsLo0Needed: isLoopback,
			IsMTU:       isMTU,
			MTU:         mtu,
		}
		if isLag {
			port.MemberPorts = []string{atePort.Name}
			port.Name = atePort.Name + "agg"
		}
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, &port)
	}

	atePort1.configureATEBGP(t, top)
	atePort2.configureATEBGP(t, top)
	atePort3.configureATEBGP(t, top)
	atePort4.configureATEIBGP(t, top, ate)

	if operation == "alterBGPAttributes" {
		t.Logf("Pushing config to Setup  Altering BGP Attributes ")
		alterBGPAttributes(t, ate, top)
	}

	configureTrafficFlow(t, top, fmt.Sprintf(`%sdst%d`, atePort1.Name, 1)+".IPv4", fmt.Sprintf(`%sdst1`, atePort2.Name)+".IPv4", atePort1.MAC, atePort1.IPv4, ipv4Traffic, ipv6Traffic)
	ate.OTG().PushConfig(t, top)
	t.Logf("Sleep for 1 minutes before starting protocols")
	time.Sleep(60 * time.Second)
	t.Logf("Wakeup from sleep and start protocols")

	start = time.Now()
	ate.OTG().StartProtocols(t)
	time.Sleep(60 * time.Second)
}

// dutSetup configures the DUT for the test.
func dutSetup(t *testing.T, dut *ondatra.DUTDevice, top gosnappi.Config, ate *ondatra.ATEDevice, nbrList []*bgpNeighbor, operation string, isMSSTest bool) {
	t.Helper()

	t.Logf("Reset DUT Config")
	if dut.Vendor() == ondatra.JUNIPER {
		t.Logf("Reset DUT Config for Juniper")
		gnmi.Delete(t, dut, gnmi.OC().Config())
	}

	defaultNetworkInstance = deviations.DefaultNetworkInstance(dut)
	dut2Data.isisData.NetworkInstanceName = defaultNetworkInstance

	for i := 1; i <= int(dutPort4.numSubIntf); i++ {
		ip4, eMsg := dutPort4.ip4(i)
		if eMsg != "" {
			t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
		}
		ip6, eMsg := dutPort4.ip6(i)
		if eMsg != "" {
			t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
		}
		dut2Data.lags[0].SubInterfaces = append(dut2Data.lags[0].SubInterfaces, &cfgplugins.DUTSubInterfaceData{
			VlanID:        vlanID(i),
			IPv4Address:   net.ParseIP(ip4),
			IPv6Address:   net.ParseIP(ip6),
			IPv4PrefixLen: 30,
			IPv6PrefixLen: 126,
		})
	}
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configure DUT", func(t *testing.T) {
		configureDUTLoopback(t, dut)

		t.Logf("===========Configuring Common BGP Policies ===========")
		configPolicy := cfgplugins.ConfigureCommonBGPPolicies(t, dut)
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), configPolicy)

		configureDUT(t, dut, &dut2Data)

		t.Logf("===========Configuring BGP  ===========")
		dutConf := configureDUTBGP(t, dutAS, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

		if weightNHConfig != "" {
			weightNHConfig += distanceConfigv4
			weightNHConfig += distanceConfigv6

			helpers.GnmiCLIConfig(t, dut, weightNHConfig)
			cfgplugins.DeviationCiscoRoutingPolicyBGPToISIS(t, dut, dutAS, "BGP", "TAG_7", "SNH", 7)
		}

		if dut.Vendor() == ondatra.JUNIPER {
			helpers.GnmiCLIConfig(t, dut, juniperLimitation)
		}
	})

	ateSetup(t, dut, top, ate, nbrList, operation, isMSSTest)
}

// verifyPortsUp verifies that all physical ports are up and the aggregate port is up.
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()

	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
	status := gnmi.Get(t, dev, gnmi.OC().Interface(aggIDs[len(aggIDs)-1]).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; status != want {
		t.Errorf("%s Status: got %v, want %v", aggIDs[len(aggIDs)-1], status, want)
	}
}

// validateLinkBWNotAdvertised validates that link bandwidth is not advertised.
func validateLinkBWNotAdvertised(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor) {
	devName := fmt.Sprintf("%sdst%d.Dev.%sdst%d.IPv4.BGP4.peer%d", atePort1.Name, 1, atePort1.Name, 1, 1)
	_, ok := gnmi.WatchAll(t,
		ate.OTG(),
		gnmi.OTG().BgpPeer(devName).UnicastIpv4PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	if ok {
		bgpPrefixes := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().BgpPeer(devName).UnicastIpv4PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.GetAddress() == "164.100.100.0" || bgpPrefix.GetAddress() == "164.100.100.128" {
				t.Logf("Prefix recevied on OTG is correct, got  Address %s, want prefix %v", bgpPrefix.GetAddress(), bgpPrefix.GetAddress())
				if len(bgpPrefix.ExtendedCommunity) == 0 {
					t.Logf("validateLinkBWNotAdvertised: Extended community is  empty. Test passed")
					return
				}
				t.Logf("validateLinkBWNotAdvertised: Extended community is  not empty: %v. Test failed", bgpPrefix.ExtendedCommunity)
			}
		}
	}

}

// verifyBgpTelemetry verifies BGP telemetry.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	counter := 0
	counter2 := 0
	for i := 1; i <= 4; i++ {
		counter = 0
		counter2 = 0
		results := gnmi.LookupAll(t, dut, bgpPath.NeighborAny().SessionState().State())

		t.Logf("results length: %v", len(results))
		for _, result := range results {
			state, ok := result.Val()
			if ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED {
				counter++
			} else {
				counter2++
			}
		}
		if counter == int(atePort2.numSubIntf+atePort3.numSubIntf+atePort4.numSubIntf+1)*2 {
			t.Logf("Break from Loop:Iteration %d: Number of established BGP sessions: %v %v. Configured BGP sessions: %d", i, counter, counter2, int(atePort2.numSubIntf+atePort3.numSubIntf+atePort4.numSubIntf+1)*2)
			break
		}
		t.Logf("Iteration %d: Number of established BGP sessions: UP %v DOWN %v Enabled BGP sessions: %d", i, counter, counter2, int(atePort2.numSubIntf+atePort3.numSubIntf+atePort4.numSubIntf+1)*2)
		time.Sleep(30 * time.Second)
	}
	if counter < int(atePort2.numSubIntf+atePort3.numSubIntf+atePort4.numSubIntf+1)*2 {
		t.Errorf("V4/V6 BGP adjacencies reported mismatch: got %v, want %v", counter, int(atePort2.numSubIntf+atePort3.numSubIntf+atePort4.numSubIntf+1)*2)
	}
}

// verifyBGPCapabilities verifies BGP capabilities.
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor) {
	t.Helper()

	t.Log("Verifying BGP capabilities.")

	nbrIP := []string{"192.0.2.2", "102.1.2.2", "153.1.2.2", "203.0.113.1", "2001:db8::192:0:2:2", "1002::102:1:2:2", "1003::153:1:2:2", "2001:db8::203:0:113:1"}

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	testBGPCapabilities := make([]func(testing.TB), len(nbrIP))

	for i, nbr := range nbrIP {
		testBGPCapabilities[i] = func(t testing.TB) {
			nbrPath := statePath.Neighbor(nbr)
			capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
				oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH:    false,
				oc.BgpTypes_BGP_CAPABILITY_MPBGP:            false,
				oc.BgpTypes_BGP_CAPABILITY_ASN32:            false,
				oc.BgpTypes_BGP_CAPABILITY_GRACEFUL_RESTART: false,
			}
			for _, sCap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
				capabilities[sCap] = true
			}
			for sCap, present := range capabilities {
				if !present {
					t.Errorf("Capability not reported: %v", sCap)
				}
			}
		}
	}

	testt.ParallelFatal(t, testBGPCapabilities...)
}

// verifyISISTelemetry verifies IS-IS telemetry.
func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, nbrList []*bgpNeighbor) {
	t.Helper()
	dutIntfs = []string{}
	var aggs1 []*oc.Interface
	if len(aggs) > 1 {
		aggs1 = aggs[len(aggs)-1:]
	} else {
		aggs1 = aggs
	}

	for _, dutPort := range aggs1 {
		if dutPort4.numSubIntf == 1 {
			dutIntfs = append(dutIntfs, dutPort.GetName())
		} else {
			for i := uint32(1); i <= dutPort4.numSubIntf; i++ {
				dutIntfs = append(dutIntfs, dutPort.GetName()+fmt.Sprintf(".%d", uint32(dutPort4.index*10)+i))
			}
		}
	}

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(dut)).Isis()
	counter := 0
	counter2 := 0
	for i := 1; i <= 4; i++ {
		counter = 0
		counter2 = 0
		results := gnmi.LookupAll(t, dut, statePath.InterfaceAny().LevelAny().AdjacencyAny().AdjacencyState().State())
		t.Logf("results length: %v", len(results))
		for _, result := range results {
			state, ok := result.Val()
			if ok && state == oc.Isis_IsisInterfaceAdjState_UP {
				counter++
			} else {
				counter2++
			}
		}
		if counter2 == 0 || counter == int(atePort4.numSubIntf) {
			t.Logf("Break from Loop:Iteration %d: Number of established IS-IS sessions: UP %v  DOWN %v Configured %v", i, counter, counter2, atePort4.numSubIntf)
			break
		}
		t.Logf("Iteration %d: Number of established IS-IS sessions: %v %v", i, counter, counter2)
		time.Sleep(30 * time.Second)
	}
	if counter != int(atePort4.numSubIntf) {
		t.Errorf("IS-IS adjacencies reported mismatch: got %v, want %v Configured %v", counter, atePort4.numSubIntf, len(dutIntfs))
	}
}

// verifyMSS verifies the MSS value for the BGP neighbors.
func verifyMSS(t *testing.T, dut *ondatra.DUTDevice, nbrList []*bgpNeighbor, dutPorts []attributes) {
	t.Logf("==================MTU Config: verifyMSS")
	var intfMTU uint16
	var ipv4 string
	var ipv6 string
	for _, dutPort := range dutPorts {
		if dutPort.Name == "port4" {
			ipv4 = "203.0.113.1"
			ipv6 = "2001:db8::203:0:113:1"
		} else if dutPort.Name == "port1" {
			ipv4 = atePort1.IPv4
			ipv6 = atePort1.IPv6
		}
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		if !deviations.SkipTCPNegotiatedMSSCheck(dut) {
			t.Run("Verify that the default TCP MSS value is set below the default interface MTU value.", func(t *testing.T) {
				// Fetch interface MTU value to compare negotiated tcp mss.
				switch {
				case deviations.OmitL2MTU(dut):
					if dutPort.Name == "port4" {
						intfMTU = gnmi.Get(t, dut, gnmi.OC().Interface(aggs[0].GetName()).Subinterface(0).Ipv4().Mtu().State())
					} else if dutPort.Name == "port1" {
						intfMTU = gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, dutPort.Name).Name()).Subinterface(0).Ipv4().Mtu().State())
					}
				default:
					if dutPort.Name == "port4" {
						intfMTU = gnmi.Get(t, dut, gnmi.OC().Interface(aggs[0].GetName()).Mtu().State())
					} else if dutPort.Name == "port1" {
						intfMTU = gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, dutPort.Name).Name()).Mtu().State())
					}
				}

				if gotTCPMss := gnmi.Get(t, dut, dutConfPath.Bgp().Neighbor(ipv4).Transport().TcpMss().State()); gotTCPMss > intfMTU || gotTCPMss == 0 {
					t.Errorf("TCP MSS for BGP v4 peer %v on dut is not as expected, got is %v, want non zero and less than %v", ipv4, gotTCPMss, intfMTU)
				} else {
					t.Logf("Test passed: TCP MSS for BGP v4 peer %v on dut is as expected, got is %v, want non zero and less than %v", ipv4, gotTCPMss, intfMTU)
				}
				if gotTCP6Mss := gnmi.Get(t, dut, dutConfPath.Bgp().Neighbor(ipv6).Transport().TcpMss().State()); gotTCP6Mss > intfMTU || gotTCP6Mss == 0 {
					t.Errorf("TCP MSS for BGP v6 peer %v on dut is not as expected, got is %v, want non zero and less than %v", ipv6, gotTCP6Mss, intfMTU)
				} else {
					t.Logf("Test passed: TCP MSS for BGP v6 peer %v on dut is as expected, got is %v, want non zero and less than %v", ipv6, gotTCP6Mss, intfMTU)
				}
			})
		}
	}
}

// setupNeighborList returns the neighbor list for the given list of ports.
func setupNeighborList(listPort []attributes) []*bgpNeighbor {
	var nbrList []*bgpNeighbor
	for _, port := range listPort {
		nbrList = append(nbrList, port.setupNbrList()...)
	}
	return nbrList
}

func (a *attributes) setupNbrList() []*bgpNeighbor {
	var nbrList []*bgpNeighbor
	nbrPeers := 0
	switch a.Name {
	case "port1":
		nbrPeers = int(dutPort1.numSubIntf)
	case "port2":
		nbrPeers = int(dutPort2.numSubIntf)
	case "port3":
		nbrPeers = int(dutPort3.numSubIntf)
	case "port4":
		nbrPeers = int(dutPort4.numSubIntf)
	}
	for i := 1; i <= nbrPeers; i++ {
		ip4, _ := a.ip4(i)
		ip6, _ := a.ip6(i)
		if a.ip4 != nil && a.Name != "port4" {
			bgpNbr := &bgpNeighbor{
				as:           ateAS4Byte((i-1)/4+1) + uint32(a.index),
				neighborip:   ip4,
				isV4:         true,
				pg:           fmt.Sprintf("%s%d", a.pg4, (i-1)/4+1),
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		} else if a.ip4Loopback != nil && i <= (1+nbrPeers/4)*2 {
			loopbackV4, eMsg := a.ip4Loopback(i)
			if eMsg != "" {
				fmt.Printf("Error in fetching IPV4 loopback address for port %s: %s", a.Name, eMsg)
			}
			bgpNbr := &bgpNeighbor{
				as:           64500,
				neighborip:   loopbackV4,
				isV4:         true,
				pg:           peervRR4GrpName,
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		} else if a.ip4Loopback != nil && i > (1+nbrPeers/4)*2 {
			loopbackV4, eMsg := a.ip4Loopback(i)
			if eMsg != "" {
				fmt.Printf("Error in fetching IPV4 loopback address for port %s: %s", a.Name, eMsg)
			}
			bgpNbr := &bgpNeighbor{
				as:           64500,
				neighborip:   loopbackV4,
				isV4:         true,
				pg:           peervVF4GrpName,
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		if a.ip6 != nil && a.Name != "port4" {
			bgpNbr := &bgpNeighbor{
				as:           ateAS4Byte((i-1)/4+1) + uint32(a.index),
				neighborip:   ip6,
				isV4:         false,
				pg:           fmt.Sprintf("%s%d", a.pg6, (i-1)/4+1),
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		} else if a.ip6Loopback != nil && i <= (1+nbrPeers/4)*2 {
			loopbackV6, eMsg := a.ip6Loopback(i)
			if eMsg != "" {
				fmt.Printf("Error in fetching IPV6 loopback address for port %s: %s", a.Name, eMsg)
			}
			bgpNbr := &bgpNeighbor{
				as:           64500,
				neighborip:   loopbackV6,
				isV4:         false,
				pg:           peervRR6GrpName,
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		} else if a.ip6Loopback != nil && i > (1+nbrPeers/4)*2 {
			loopbackV6, eMsg := a.ip6Loopback(i)
			if eMsg != "" {
				fmt.Printf("Error in fetching IPV6 loopback address for port %s: %s", a.Name, eMsg)
			}
			bgpNbr := &bgpNeighbor{
				as:           64500,
				neighborip:   loopbackV6,
				isV4:         false,
				pg:           peervVF6GrpName,
				importPolicy: a.importPolicy,
				exportPolicy: a.exportPolicy,
			}
			nbrList = append(nbrList, bgpNbr)
		}
	}
	return nbrList
}

// bgpEvents performs the BGP events and measures the convergence time.
func bgpEvents(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, nbrList []*bgpNeighbor, dutInfoPreReset dutInfo, operation string, v4ConvergenceCount, v6ConvergenceCount uint32) {
	t.Helper()
	dutInfoPostReset := dutInfo{processes: make(map[string]processesInfo)}

	dutInfoPreReset = verifySystemHealth(t, dut)
	cs := gosnappi.NewControlState()

	switch operation {
	case "resetBGPPeers":
		t.Logf("%v: Tear down BGP peers", operation)
		cs.Protocol().Bgp().Peers().SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
		ate.OTG().SetControlState(t, cs)
		t.Logf("%v: Bring down BGP peers and Sleep for 90 seconds", operation)
		time.Sleep(time.Second * 90)
		t.Logf("%v: Bring down BGP peers:  Wake up from 90 seconds sleep", operation)

		t.Logf("%v: Bring up BGP peers and Sleep for 90 seconds", operation)
		cs.Protocol().Bgp().Peers().SetState(gosnappi.StateProtocolBgpPeersState.UP)
		ate.OTG().SetControlState(t, cs)
		time.Sleep(time.Second * 90)
		t.Logf("%v: Bring up BGP peers: Wake up from 90 seconds sleep", operation)

	case "withdrawRoutes":
		cs.Protocol().Route().SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
		t.Logf("%v: Withdraw routes", operation)
		start = time.Now()
		ate.OTG().SetControlState(t, cs)
		measureConvergence(t, dut, []string{"ipv4", "ipv6"}, v4ConvergenceCount, v6ConvergenceCount, true)

		cs.Protocol().Route().SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
		t.Logf("%v: Advertise routes", operation)
		ate.OTG().SetControlState(t, cs)

	case "alterBGPAttributes":
		t.Logf("%v: Alter BGP attributes", operation)
		ate.Topology().New()
		top = gosnappi.NewConfig()

		ateSetup(t, dut, top, ate, nbrList, "alterBGPAttributes", isMSSTest)
		t.Logf("%v: Alter BGP attributes completed", operation)
		time.Sleep(time.Second * 120)

	default:
		t.Errorf("ERROR: Unsupported operation: %s", operation)
	}

	t.Logf(" %s completed. Measuring DUT Convergence", operation)
	start = time.Now()
	measureConvergence(t, dut, []string{"ipv4", "ipv6"}, v4ConvergenceCount, v6ConvergenceCount, false)

	t.Logf("%s: Verify ISIS and BGP Telemetry", operation)
	verifyISISTelemetry(t, dut, ate, top, nbrList)

	t.Logf("%s: Verify BGP Telemetry", operation)
	verifyBgpTelemetry(t, dut, ate, top, nbrList)

	t.Logf("%s: Verify traffic ", operation)
	sendTraffic(t, ate, top)
	verifyTraffic(t, ate, top)

	t.Logf("%s: Check DUT health", operation)
	dutInfoPostReset = verifySystemHealth(t, dut)

	for procName := range dutInfoPostReset.processes {
		if slices.Contains([]string{"bgp", "bgp-main", "rpd", "sr_bgp_mgr"}, strings.ToLower(procName)) {
			t.Logf("Pre Reset: ProcessName: %v, PID: %v, CPU: %v, Memory: %v", procName, dutInfoPreReset.processes[procName].pid, dutInfoPreReset.processes[procName].cpuPct, dutInfoPreReset.processes[procName].memPct)
			t.Logf("Post Reset: ProcessName: %v, PID: %v, CPU: %v, Memory: %v", procName, dutInfoPostReset.processes[procName].pid, dutInfoPostReset.processes[procName].cpuPct, dutInfoPostReset.processes[procName].memPct)
		}

		if _, ok := dutInfoPreReset.processes[procName]; dutInfoPostReset.processes[procName].pid != dutInfoPreReset.processes[procName].pid && ok {
			t.Logf("Process: %v restarted between initial and final health check: new pid: %v, old pid: %v", procName, dutInfoPostReset.processes[procName].pid, dutInfoPreReset.processes[procName].pid)
		}
	}
	t.Logf("Verify Final system health")
}

func alterBGPAttributes(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	devices := top.Devices().Items()
	for _, device := range devices {
		if len(device.Bgp().Ipv4Interfaces().Items()) == 0 || len(device.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()) == 0 {
			continue // No BGP peer to configure
		}
		bgp4Peer := device.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
		updates := bgp4Peer.ReplayUpdates().StructuredPdus().Updates()
		deviceName := device.Name()

		if len(device.Ethernets().Items()) == 0 || len(device.Ethernets().Items()[0].Ipv4Addresses().Items()) == 0 {
			t.Fatalf("Device %s missing expected Ethernet or IPv4 configuration", deviceName)
			continue
		}
		nextHopV4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Address()
		commonCommunities := map[uint32][]string{300: otgconfighelpers.GenerateCommunityStrings(8, "10%d")}
		longASPath := []uint32{800101, 800111, 800121, 800131, 800141, 800151, 800161, 800171, 800181, 800191,
			8001101, 8001111, 8001121, 8001131, 8001141, 8001151, 8001161, 8001171, 8001181, 8001191,
			80011101, 80011111, 80011121, 80011131, 80011141, 80011151, 80011161, 80011171, 80011181, 80011191}

		if strings.Contains(deviceName, "port2") {
			routes := otgconfighelpers.GenerateIPv4Prefixes(80, []otgconfighelpers.PrefixFormat{
				{Format: "122.100.%d.0", Length: 24},
				{Format: "123.%d.0.0", Length: 16},
			})
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				Origin: gosnappi.BgpAttributesOrigin.IGP, ASPath: []uint32{800101}, Communities: commonCommunities, NextHop: nextHopV4, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				TimeGap: 120000, Origin: gosnappi.BgpAttributesOrigin.INCOMPLETE, ASPath: longASPath, Communities: commonCommunities, NextHop: nextHopV4, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPWithdrawal(updates, otgconfighelpers.WithdrawalOptions{TimeGap: 60000, IPv4Routes: routes})
		} else if strings.Contains(deviceName, "port3") {
			routes := otgconfighelpers.GenerateIPv4Prefixes(80, []otgconfighelpers.PrefixFormat{
				{Format: "124.100.%d.0", Length: 24},
				{Format: "125.%d.0.0", Length: 16},
			})
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				Origin: gosnappi.BgpAttributesOrigin.IGP, ASPath: []uint32{800101}, Communities: commonCommunities, NextHop: nextHopV4, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				TimeGap: 120000, Origin: gosnappi.BgpAttributesOrigin.INCOMPLETE, ASPath: longASPath, Communities: commonCommunities, NextHop: nextHopV4, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPWithdrawal(updates, otgconfighelpers.WithdrawalOptions{TimeGap: 120000, IPv4Routes: routes})
		} else if strings.Contains(deviceName, "port4") {
			routes := otgconfighelpers.GenerateIPv4Prefixes(20, []otgconfighelpers.PrefixFormat{
				{Format: "111.100.%d.0", Length: 25}, {Format: "111.100.%d.32", Length: 25},
				{Format: "111.111.%d.64", Length: 27}, {Format: "111.111.%d.96", Length: 27},
				{Format: "111.111.%d.128", Length: 27}, {Format: "111.111.%d.160", Length: 27},
				{Format: "111.111.%d.192", Length: 27}, {Format: "111.111.%d.224", Length: 27},
			})
			lp102 := uint32(102)
			lp1000 := uint32(1000)
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				Origin: gosnappi.BgpAttributesOrigin.IGP, ASPath: []uint32{800101}, Communities: commonCommunities, NextHop: "50.1.1.2", LocalPref: &lp102, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPAdvertisement(updates, otgconfighelpers.AdvertisementOptions{
				TimeGap: 120000, Origin: gosnappi.BgpAttributesOrigin.INCOMPLETE, ASPath: []uint32{800101, 800111}, Communities: commonCommunities, NextHop: "50.1.2.2", LocalPref: &lp1000, IPv4Routes: routes,
			})
			otgconfighelpers.AddBGPWithdrawal(updates, otgconfighelpers.WithdrawalOptions{TimeGap: 120000, IPv4Routes: routes})
		}
	}
}

// createPrefixesV4 creates the prefixes for V4.
func createPrefixesV4(t *testing.T, portIndex int, groupIndex int) []netip.Prefix {
	t.Helper()

	var ips []netip.Prefix
	for i := 30; i < 32; i++ {
		for j := 252; j < 256; j += 4 {
			if 10*portIndex+((groupIndex-1)/4)*4 > 254 {
				continue
			}
			if netip.MustParsePrefix(fmt.Sprintf("%d.%d.%d.0/22", 10*portIndex+((groupIndex-1)/4)*4, i, j)).IsValid() {
				ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("%d.%d.%d.0/22", 10*portIndex+((groupIndex-1)/4)*4, i, j)))
				ip := fmt.Sprintf("%d.%d.%d.0", 10*portIndex+((groupIndex-1)/4)*4, i, j)
				if !slices.Contains(ipv4Traffic[portIndex], ip) && (portIndex == 2 || portIndex == 3) {
					ipv4Traffic[portIndex] = append(ipv4Traffic[portIndex], ip)
				}
			}
		}
	}

	for i := 39; i < 41; i++ {
		for j := 251; j < 250; j++ {
			if 20*portIndex+((groupIndex-1)/4)*4 > 254 {
				continue
			}
			if netip.MustParsePrefix(fmt.Sprintf("%d.%d.%d.0/24", 20*portIndex+((groupIndex-1)/4)*4, i, j)).IsValid() {
				ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("%d.%d.%d.0/24", 20*portIndex+((groupIndex-1)/4)*4, i, j)))
			}
		}
	}

	for i := 183; i < 184; i++ {
		for j := 252; j < 256; j += 4 {
			ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("175.41.%d.%d/30", i, j)))
		}
	}

	return ips
}

// createPrefixesV6 creates the prefixes for V6.
func createPrefixesV6(t *testing.T, portIndex int) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix
	for i := 118; i < 120; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("2001:db8:%d:%d::/48", portIndex, i))
		ips = append(ips, ip)
		if !slices.Contains(ipv6Traffic[portIndex], ip.Addr().String()) && (portIndex == 2 || portIndex == 3) {
			ipv6Traffic[portIndex] = append(ipv6Traffic[portIndex], ip.Addr().String())
		}
	}

	for i := 358; i < 360; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("fc00:abcd:%d:1:1:%d::/64", portIndex, i))
		ips = append(ips, ip)
	}

	for i := 1919; i < 1920; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("fc00:abcd:%d::%d:1/126", portIndex, i))
		ips = append(ips, ip)
	}

	return ips
}

// incrementMAC increments the MAC address by the given value.
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

func constructBGPPeerGroups(dutPorts []attributes) []bgpGroupConfig {
	var bgpPeerGroup []bgpGroupConfig

	for _, port := range dutPorts {
		transport := ""
		var pgAFISAFIType oc.E_BgpTypes_AFI_SAFI_TYPE
		if port.Name == "port4" {
			for _, name := range port.pgList {
				isV4 := strings.Contains(name, "V4")
				isV6 := strings.Contains(name, "V6")
				if strings.Contains(name, "V4") {
					pgAFISAFIType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
				} else {
					pgAFISAFIType = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
				}
				if isV4 {
					transport = dutLoopback.IPv4
				} else {
					transport = dutLoopback.IPv6
				}

				bgpPeerGroup = append(bgpPeerGroup, bgpGroupConfig{
					pgName:           name,
					pgAS:             uint32(64500),
					pgHoldTimer:      defaultHoldTimer,
					pgKeepaliveTimer: defaultKeepaliveTimer,
					pgUpdateTimer:    defaultUpdateTimer,
					enablePGGR:       defaultEnablePGGR,
					pgV4Unicast:      isV4,
					pgV6Unicast:      isV6,
					pgAFISAFI:        pgAFISAFIType,
					pgImportPolicy:   port.importPolicy,
					pgExportPolicy:   port.exportPolicy,
					pgTransport:      transport,
					pgMultipath:      false,
				})
			}
		} else {
			for _, name := range port.pgList {
				isV4 := strings.Contains(name, "V4")
				isV6 := strings.Contains(name, "V6")
				if strings.Contains(name, "V4") {
					pgAFISAFIType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
				} else {
					pgAFISAFIType = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
				}
				for i := 1; i <= int(port.numSubIntf); i++ {
					groupIndex := (i-1)/4 + 1
					bgpPeerGroup = append(bgpPeerGroup, bgpGroupConfig{
						pgName:           fmt.Sprintf("%s%d", name, groupIndex),
						pgAS:             ateAS4Byte(groupIndex) + uint32(port.index),
						pgHoldTimer:      defaultHoldTimer,
						pgKeepaliveTimer: defaultKeepaliveTimer,
						pgUpdateTimer:    defaultUpdateTimer,
						enablePGGR:       defaultEnablePGGR,
						pgV4Unicast:      isV4,
						pgV6Unicast:      isV6,
						pgAFISAFI:        pgAFISAFIType,
						pgImportPolicy:   port.importPolicy,
						pgExportPolicy:   port.exportPolicy,
						pgTransport:      transport,
						pgMultipath:      true,
					})
				}
			}
		}
	}
	return bgpPeerGroup
}

func configureDUTBGP(t *testing.T, as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	bgpPeerGroups := constructBGPPeerGroups([]attributes{dutPort1, dutPort2, dutPort3, dutPort4})
	afi := ""
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	// Global Configuration using cfgplugins
	cfgplugins.ConfigureGlobal(bgp, dut,
		cfgplugins.WithAS(as),
		cfgplugins.WithRouterID(dutLoopback.IPv4),
		cfgplugins.WithGlobalGracefulRestart(true, 120, 300),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, true),
		cfgplugins.WithExternalRouteDistance(9),
		cfgplugins.WithGlobalEBGPMultipath(64),
	)
	if deviations.BgpDistanceOcPathUnsupported(dut) {
		distanceConfigv4 = fmt.Sprintf("router bgp %d instance BGP\naddress-family %s unicast\ndistance bgp %d 200 200\n", dutAS, "ipv4", 9)
		distanceConfigv6 = fmt.Sprintf("router bgp %d instance BGP\naddress-family %s unicast\ndistance bgp %d 200 200\n", dutAS, "ipv6", 9)
	}

	for _, pg := range bgpPeerGroups {
		if pg.pgV4Unicast {
			afi = "ipv4"
		} else {
			afi = "ipv6"
		}
		if dut.Vendor() == ondatra.CISCO {
			if weightNHConfig == "" {
				weightNHConfig = fmt.Sprintf("router bgp %d instance BGP\n", dutAS)
			}
			weightNHConfig += fmt.Sprintf("neighbor-group %s\n address-family %s unicast\n weight 49151\n", pg.pgName, afi)
			if pg.pgMultipath {
				weightNHConfig += fmt.Sprintf("neighbor-group %s\n dmz-link-bandwidth\nebgp-recv-extcommunity-dmz\nebgp-send-extcommunity-dmz\n", pg.pgName)
			} else {
				weightNHConfig += fmt.Sprintf("neighbor-group %s\n address-family %s unicast\n weight 49151\n", pg.pgName, afi)
				weightNHConfig += fmt.Sprintf("neighbor-group %s\n address-family %s unicast\n next-hop-unchanged multipath\n", pg.pgName, afi)
			}
		}

		peerGroup := bgp.GetOrCreatePeerGroup(pg.pgName)
		cfgplugins.ConfigurePeerGroup(peerGroup, dut,
			cfgplugins.WithPeerAS(pg.pgAS),
			cfgplugins.WithPGTimers(pg.pgHoldTimer, pg.pgKeepaliveTimer, pg.pgUpdateTimer),
			cfgplugins.WithPGDescription(pg.pgName),
			cfgplugins.WithPGGracefulRestart(pg.enablePGGR),
			cfgplugins.WithPGTransport(pg.pgTransport),
			cfgplugins.WithPGMultipath(pg.pgName, pg.pgMultipath),
			cfgplugins.WithPGSendCommunity([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED}),
			cfgplugins.WithPGAfiSafiEnabled(pg.pgAFISAFI, true, true),
			cfgplugins.ApplyPGRoutingPolicy(pg.pgImportPolicy, pg.pgExportPolicy, dut.Vendor() == ondatra.CISCO || delLinkbwCLIConfig != ""),
		)
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		cfgplugins.ConfigurePeer(bgpNbr, dut,
			cfgplugins.WithPeerGroup(nbr.pg, uint32(nbr.as), nbr.importPolicy, nbr.exportPolicy, delLinkbwCLIConfig != "" && dut.Vendor() == ondatra.CISCO),
			cfgplugins.WithPeerAfiSafiEnabled(nbr.isV4, nbr.importPolicy, nbr.exportPolicy, delLinkbwCLIConfig != "" && dut.Vendor() == ondatra.CISCO),
			cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(nbr.isV4, nbr.importPolicy, nbr.exportPolicy, delLinkbwCLIConfig != "" && dut.Vendor() == ondatra.CISCO),
		)
	}
	return niProto
}

// configureDUTLoopback configures the DUT loopback interface.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lb = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	lo1 := dutLoopback.NewOCInterface(lb, dut)
	lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		if dut.Vendor() != ondatra.NOKIA {
			fptest.AssignToNetworkInstance(t, dut, lb, deviations.DefaultNetworkInstance(dut), 0)
		} else if dut.Vendor() == ondatra.NOKIA && (slices.Contains(kneDeviceModelList, dut.Model()) || dut.Model() == "ixr10e") {
			AssignToNetworkInstance(t, dut, lb, deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

// AssignToNetworkInstance assigns the interface to the network instance.
func AssignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	netInstIntf.Id = ygot.String(intf.GetName())
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

// configureTrafficFlow configures the traffic flow.
func configureTrafficFlow(t *testing.T, otgConfig gosnappi.Config, flowSrcEndPoint, flowDstEndPoint, srcMac, srcIP string, dstIPs map[int][]string, dstIPsV6 map[int][]string) {
	t.Helper()

	otgConfig.Flows().Clear()

	for portIndex, dstIP := range dstIPs {
		if portIndex == 0 || portIndex == 1 {
			continue
		}
		flowDstEndPoint := flowSrcEndPoint
		if portIndex == 2 {
			flowDstEndPoint = fmt.Sprintf(`%sdst1`, atePort2.Name) + ".IPv4"
		} else if portIndex == 3 {
			flowDstEndPoint = fmt.Sprintf(`%sdst%d`, atePort3.Name, portIndex) + ".IPv4"
		}

		if len(dstIP) > 16 {
			dstIP = dstIP[:16]
		}

		for i, dstip := range dstIP {

			flow := otgConfig.Flows().Add().SetName("V4Flow_" + fmt.Sprintf(`%d%d`, portIndex, i))
			flow.Metrics().SetEnable(true)
			flow.TxRx().Device().
				SetTxNames([]string{flowSrcEndPoint}).
				SetRxNames([]string{flowDstEndPoint})
			flow.Size().SetFixed(1500)
			flow.Duration().FixedPackets().SetPackets(1000)
			e := flow.Packet().Add().Ethernet()
			e.Src().SetValue(srcMac)

			v4 := flow.Packet().Add().Ipv4()
			v4.Src().SetValue(srcIP)
			v4.Dst().Increment().SetStart(dstip).SetStep("0.0.0.1").SetCount(100)
			tcp := flow.Packet().Add().Tcp()
			tcp.DstPort().Increment().SetStart(12345).SetCount(200)
		}
	}
	for portIndex, dstIP := range dstIPsV6 {
		if portIndex == 0 || portIndex == 1 {
			continue
		}
		flowDstEndPoint := fmt.Sprintf(`%sdst1`, atePort1.Name) + ".IPv6"
		if portIndex == 2 {
			flowDstEndPoint = fmt.Sprintf(`%sdst1`, atePort2.Name) + ".IPv6"
		} else if portIndex == 3 {
			flowDstEndPoint = fmt.Sprintf(`%sdst%d`, atePort3.Name, portIndex) + ".IPv6"
		}

		for i, dstip := range dstIP {

			flow := otgConfig.Flows().Add().SetName("V6Flow_" + fmt.Sprintf(`%d%d`, portIndex, i))
			flow.Metrics().SetEnable(true)
			flow.TxRx().Device().
				SetTxNames([]string{flowSrcEndPoint}).
				SetRxNames([]string{flowDstEndPoint})
			flow.Size().SetFixed(1500)
			flow.Duration().FixedPackets().SetPackets(1000)
			e := flow.Packet().Add().Ethernet()
			e.Src().SetValue(srcMac)
			v6 := flow.Packet().Add().Ipv6()
			v6.Src().SetValue(atePort1.IPv6)
			v6.Dst().Increment().SetStart(dstip).SetStep("0:0:0:0:0:0:0:1").SetCount(100)
			tcp := flow.Packet().Add().Tcp()
			tcp.DstPort().Increment().SetStart(12345).SetCount(200)
		}
	}
}

// sendTraffic sends traffic from the ATE.
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)
}

// verifyTraffic verifies the traffic flow.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range conf.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("TxPkts = 0, want > 0")
		}
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > tolerancePct {
			t.Fatalf("Traffic Loss Pct for Flow %s: got %v, want max %v pct failure", flow.Name(), lossPct, tolerancePct)
		} else {
			t.Logf("Traffic Test Passed! for flow %s", flow.Name())
		}
	}
}

// verifySystemHealth verifies the system health.
func verifySystemHealth(t *testing.T, dut *ondatra.DUTDevice) dutInfo {
	t.Helper()
	t.Logf("INFO: Verifying system health")
	var dutInfo dutInfo

	t.Logf("INFO: Check CPU utilization")
	query := gnmi.OC().System().ProcessAny().State()
	results := gnmi.GetAll(t, dut, query)

	for _, result := range results {
		processName := result.GetName()
		if dutInfo.processes == nil {
			dutInfo.processes = make(map[string]processesInfo)
		}
		dutInfo.processes[processName] = processesInfo{
			pid:    result.GetPid(),
			cpuPct: result.GetCpuUtilization(),
			memPct: result.GetMemoryUtilization(),
		}
	}
	return dutInfo
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutData *dutData) {
	t.Logf("===========Configuring DUT===========")
	t.Helper()
	var cfgDutPorts []cfgplugins.Attributes
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, l := range dutData.lags {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		agg := cfgplugins.NewAggregateInterface(t, dut, b, l)
		aggID := &oc.Interface{Name: ygot.String(agg.GetName())}
		aggs = append(aggs, aggID)
		aggIDs = append(aggIDs, agg.GetName())
		agg.DeleteSubinterface(0)
		b.Set(t, dut)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			for i := uint32(1); i <= dutPort4.numSubIntf; i++ {
				fptest.AssignToNetworkInstance(t, dut, aggIDs[0], deviations.DefaultNetworkInstance(dut), uint32(dutPort4.index*10)+i)
			}
		}
	}
	// Wait for LAG interfaces to AdminStatus to be UP
	for _, l := range dutData.lags {
		gnmi.Await(t, dut, gnmi.OC().Interface(l.LagName).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	}

	t.Logf("===========Configuring ISIS ===========")
	dutData.isisData.ISISInterfaceNames = createISISInterfaceNames(t, dut, dutData)
	b := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, dutData.isisData, b)
	b.Set(t, dut)

	t.Logf("===========Configuring BGP to ISIS redistribution ===========")
	cfgplugins.BgpISISRedistribution(t, dut, "ipv6", b, "BGP_TO_ISIS")
	cfgplugins.BgpISISRedistribution(t, dut, "ipv4", b, "BGP_TO_ISIS")
	b.Set(t, dut)

	for _, dp := range []attributes{dutPort1, dutPort2, dutPort3} {
		cfgDutPorts = append(cfgDutPorts, cfgplugins.Attributes{
			Attributes:  &attrs.Attributes{Name: dp.Name, IPv4: dp.IPv4, IPv4Len: v4VlanPlen, IPv6: dp.IPv6, IPv6Len: v6VlanPlen, MAC: dp.MAC, Desc: dp.Desc},
			NumSubIntf:  dp.numSubIntf,
			Index:       dp.index,
			Ip4:         dp.ip4,
			Ip6:         dp.ip6,
			Gateway:     dp.gateway,
			Gateway6:    dp.gateway6,
			Ip4Loopback: dp.ip4Loopback,
			Ip6Loopback: dp.ip6Loopback,
			LagMAC:      dp.lagMAC,
			EthMAC:      dp.ethMAC,
			Port1MAC:    dp.port1MAC,
			Pg4:         dp.pg4,
			Pg6:         dp.pg6,
		})
	}
	cfgplugins.NewSubInterfaces(t, dut, cfgDutPorts)

	// TODO: Need to investigate if we need to define a deviation for this.
	if dut.Vendor() == ondatra.ARISTA {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n no switchport \n", aggIDs[0]))
		for _, dp := range []attributes{dutPort1, dutPort2, dutPort3} {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n no switchport \n", dut.Port(t, dp.Name).Name()))
		}
	}
}

func createISISInterfaceNames(t *testing.T, dut *ondatra.DUTDevice, dt *dutData) []string {
	t.Helper()
	loopback0 := netutil.LoopbackInterface(t, dut, 0)
	interfaceNames := []string{loopback0}
	for _, l := range dt.lags {
		if l.Attributes.IPv4 != "" {
			interfaceNames = append(interfaceNames, l.LagName)
			t.Logf("First ConditioninterfaceNames: %v", interfaceNames)
		} else {
			for _, s := range l.SubInterfaces {
				interfaceNames = append(interfaceNames, fmt.Sprintf("%s.%d", l.LagName, s.VlanID))
			}
		}
	}
	t.Logf("interfaceNames: %v", interfaceNames)
	return interfaceNames
}

func (a *attributes) configureATEBGP(t *testing.T, top gosnappi.Config) {
	t.Helper()

	devices := top.Devices().Items()
	devMap := make(map[string]gosnappi.Device)
	for _, dev := range devices {
		devMap[dev.Name()] = dev
	}

	for i := 1; i <= int(a.numSubIntf); i++ {
		di := fmt.Sprintf("%sdst%d.Dev", a.Name, i)

		asn1 := ateAS4Byte((i-1)/4+1) + uint32(a.index)
		device := devMap[di]

		routerID := a.IPv4
		if a.numSubIntf > 1 && a.ip4 != nil {
			routerID, _ = a.ip4(i)
		}

		var bgp4Peer gosnappi.BgpV4Peer
		var bgp6Peer gosnappi.BgpV6Peer
		if a.ip4 != nil {
			ipv4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			bgp4Peer = otgconfighelpers.AddBGPV4Peer(device, ipv4.Name(),
				otgconfighelpers.WithBGPRouterID(routerID),
				otgconfighelpers.WithBGPPeerAddress(ipv4.Gateway()),
				otgconfighelpers.WithBGPASNumber(asn1),
				otgconfighelpers.WithBGPEBGP(),
				otgconfighelpers.WithBGPLearnedV4Pfx(true),
				// Default GR and Timers are used
			)
		}
		if a.ip6 != nil {
			ipv6 := device.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			bgp6Peer = otgconfighelpers.AddBGPV6Peer(device, ipv6.Name(),
				otgconfighelpers.WithBGPRouterID(routerID),
				otgconfighelpers.WithBGPPeerAddress(ipv6.Gateway()),
				otgconfighelpers.WithBGPASNumber(asn1),
				otgconfighelpers.WithBGPEBGP(),
				otgconfighelpers.WithBGPLearnedV6Pfx(true),
			)
		}

		if a.Name == "port1" {
			return
		}
		prefixesV4 := createPrefixesV4(t, int(a.index)+1, 1)
		for i := 2; i <= int(a.numSubIntf); i++ {
			prefixesV4 = append(prefixesV4, createPrefixesV4(t, int(a.index)+1, i)...)
		}
		prefixesV6 := createPrefixesV6(t, int(a.index)+1)
		var prefixesV4Str, prefixesV6Str []string
		for _, prefix := range prefixesV4 {
			prefixesV4Str = append(prefixesV4Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
		}
		for _, prefix := range prefixesV6 {
			prefixesV6Str = append(prefixesV6Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
		}
		var communitiesSet1, communitiesSet2 []gosnappi.BgpCommunity
		var extCommunitiesSet1 []gosnappi.BgpExtendedCommunity
		for j := 1; j < 8; j++ {
			communitiesSet1 = append(communitiesSet1, otgconfighelpers.CreateBGPCommunity(100, uint32(100)+uint32(j)))
			communitiesSet2 = append(communitiesSet2, otgconfighelpers.CreateBGPCommunity(100, uint32(100)+uint32(j)))
		}
		communitiesSet2 = append(communitiesSet2, otgconfighelpers.CreateBGPCommunity(100, 100))
		extCommunitiesSet1 = append(extCommunitiesSet1, otgconfighelpers.CreateBGPLinkBandwidthExtCommunity(64500, 1000000000))

		otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-LBW-%d%d%s", a.index, i+int(a.index*10), bgp4Peer.Name()),
			[]string{"164.100.100.0/27", "164.100.100.32/27"},
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
			otgconfighelpers.WithBGPRouteExtendedCommunities(extCommunitiesSet1),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)

		otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-LBW-%d%d%s", a.index, i+int(a.index*10), bgp6Peer.Name()),
			[]string{"2003:db8:1::/48", "2003:db8:2::/48"},
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
			otgconfighelpers.WithBGPRouteExtendedCommunities(extCommunitiesSet1),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)

		otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-%d%d%s", a.index, i+int(a.index*10), bgp4Peer.Name()),
			prefixesV4Str,
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)
		otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-%d%d%s", a.index, i+int(a.index*10), bgp6Peer.Name()),
			prefixesV6Str,
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)

		bgpCom := gosnappi.NewBgpCommunity()
		bgpCom.SetType(gosnappi.BgpCommunityType.NO_ADVERTISED)
		communitiesSet2 = append(communitiesSet2, bgpCom)
		var subnet string
		if i <= 245 {
			subnet = fmt.Sprintf("50.1.%d.0", ((i-1)/4)*4) + "/22"
		} else if i < 490 {
			subnet = fmt.Sprintf("50.4.%d.0", ((i-244-1)/4)*4) + "/22"
		} else if i >= 490 {
			subnet = fmt.Sprintf("50.6.%d.0", ((i-489-1)/4)*4) + "/22"
		}
		otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-SNH-dev%d%d%s", a.index, i+int(a.index)*10, bgp4Peer.Name()),
			[]string{subnet},
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)
		otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-SNH-dev%d%d%s", a.index, i+int(a.index)*10, bgp6Peer.Name()),
			[]string{fmt.Sprintf("2001:%d::", ((i-1)/4)*4) + "/48"},
			otgconfighelpers.WithBGPRouteNextHopMode("LOCAL_IP"),
			otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
			otgconfighelpers.WithBGPRouteAddressCount(1),
		)
	}
}

func (a *attributes) configureATEIBGP(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	devices := top.Devices().Items()
	devMap := make(map[string]gosnappi.Device)
	for _, dev := range devices {
		devMap[dev.Name()] = dev
	}

	for i := 1; i <= int(a.numSubIntf); i++ {
		di := a.Name
		if a.Name == "port2" || a.Name == "port3" || a.Name == "port4" || a.Name == "port1" {
			di = fmt.Sprintf("%sdst%d.Dev", a.Name, i)
		}
		dev := devMap[di]
		intfName := fmt.Sprintf(`%sdst%d`, a.Name, i)

		configureOTGISIS(t, "lag1.Dev", dev, atePort4, ate, i)

		// IBGP Configuration
		routerID, _ := a.ip4Loopback(i)
		// IPv4 IBGP Peer
		bgp4Peer := otgconfighelpers.AddBGPV4Peer(dev, intfName+".Loopback.IPv4",
			otgconfighelpers.WithBGPName(dev.Name()+".IBGP.BGP4.peer"+strconv.Itoa(int(i))),
			otgconfighelpers.WithBGPRouterID(routerID),
			otgconfighelpers.WithBGPPeerAddress(dutLoopback.IPv4),
			otgconfighelpers.WithBGPASNumber(64500),
			otgconfighelpers.WithBGPIBGP(),
			otgconfighelpers.WithBGPLearnedV4Pfx(true),
		)

		// IPv6 IBGP Peer
		bgp6Peer := otgconfighelpers.AddBGPV6Peer(dev, intfName+".Loopback.IPv6",
			otgconfighelpers.WithBGPName(dev.Name()+".IBGP.BGP6.peer"+strconv.Itoa(int(i))),
			otgconfighelpers.WithBGPRouterID(routerID),
			otgconfighelpers.WithBGPPeerAddress(dutLoopback.IPv6),
			otgconfighelpers.WithBGPASNumber(64500),
			otgconfighelpers.WithBGPIBGP(),
			otgconfighelpers.WithBGPLearnedV6Pfx(true),
		)

		// BGP Route Advertisements
		var communitiesSet1, communitiesSet2 []gosnappi.BgpCommunity
		for j := 1; j < 8; j++ {
			communitiesSet1 = append(communitiesSet1, otgconfighelpers.CreateBGPCommunity(200, uint32(100)+uint32(j)))
			communitiesSet2 = append(communitiesSet2, otgconfighelpers.CreateBGPCommunity(200, uint32(100)+uint32(j)))
		}
		if i <= (1+int(a.numSubIntf)/4)*2 {
			// RR Routes
			otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-RR-dev%d%d", a.index, i+int(a.index*10)),
				[]string{"12.24.0.0/25"},
				otgconfighelpers.WithBGPRouteNextHopIPv4(fmt.Sprintf("60.1.0.%d", 40+i)),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
				otgconfighelpers.WithBGPRouteAddressCount(v4IBGPRouteCount),
			)
			otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-RR-dev%d%d", a.index, i+int(a.index*10)),
				[]string{"2001:bbbb:0:1::/64"},
				otgconfighelpers.WithBGPRouteNextHopIPv6(fmt.Sprintf("2010:3:db8:a%x::1", i)),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet1),
				otgconfighelpers.WithBGPRouteAddressCount(v6IBGPRouteCount),
			)
		} else {
			// VF Routes
			prefixesV4 := createPrefixesV4(t, 2, i)
			prefixesV6 := createPrefixesV6(t, 2)
			var prefixesP2V4Str, prefixesP2V6Str, prefixesP3V4Str, prefixesP3V6Str []string
			for _, prefix := range prefixesV4 {
				prefixesP2V4Str = append(prefixesP2V4Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
			}
			for _, prefix := range prefixesV6 {
				prefixesP2V6Str = append(prefixesP2V6Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
			}
			asPathVF := otgconfighelpers.CreateBGPASPath([]uint32{ateAS4Byte(1) + uint32(atePort2.index)}, gosnappi.BgpAsPathSegmentType.AS_SEQ, gosnappi.BgpAsPathAsSetMode.DO_NOT_INCLUDE_LOCAL_AS)

			otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-VF-P2-dev%d%d", a.index, i+int(a.index*10)),
				prefixesP2V4Str,
				otgconfighelpers.WithBGPRouteNextHopIPv4("50.1.1.2"),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
				otgconfighelpers.WithBGPRouteAddressCount(1),
				otgconfighelpers.WithBGPRouteASPath(asPathVF),
			)
			otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-VF-P2-dev%d%d", a.index, i+int(a.index*10)),
				prefixesP2V6Str,
				otgconfighelpers.WithBGPRouteNextHopIPv6("1000:1:11:0:50:1:1:2"),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
				otgconfighelpers.WithBGPRouteAddressCount(1),
				otgconfighelpers.WithBGPRouteASPath(asPathVF),
			)

			prefixesV4 = createPrefixesV4(t, 3, i)
			prefixesV6 = createPrefixesV6(t, 3)
			for _, prefix := range prefixesV4 {
				prefixesP3V4Str = append(prefixesP3V4Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
			}
			for _, prefix := range prefixesV6 {
				prefixesP3V6Str = append(prefixesP3V6Str, prefix.Addr().String()+"/"+strconv.Itoa(prefix.Bits()))
			}

			asPathVF = otgconfighelpers.CreateBGPASPath([]uint32{ateAS4Byte(int(atePort3.index-1)) + uint32(atePort3.index)}, gosnappi.BgpAsPathSegmentType.AS_SEQ, gosnappi.BgpAsPathAsSetMode.DO_NOT_INCLUDE_LOCAL_AS)

			otgconfighelpers.AddBGPV4Routes(bgp4Peer, fmt.Sprintf("v4-bgpNet-VF-P3-dev%d%d", a.index, i+int(a.index*10)),
				prefixesP3V4Str,
				otgconfighelpers.WithBGPRouteNextHopIPv4("50.2.1.2"),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
				otgconfighelpers.WithBGPRouteAddressCount(1),
				otgconfighelpers.WithBGPRouteASPath(asPathVF),
			)
			otgconfighelpers.AddBGPV6Routes(bgp6Peer, fmt.Sprintf("v6-bgpNet-VF-P3-dev%d%d", a.index, i+int(a.index*10)),
				prefixesP3V6Str,
				otgconfighelpers.WithBGPRouteNextHopIPv6("1000:2:21:0:50:2:1:2"),
				otgconfighelpers.WithBGPRouteCommunities(communitiesSet2),
				otgconfighelpers.WithBGPRouteAddressCount(1),
				otgconfighelpers.WithBGPRouteASPath(asPathVF),
			)
		}
	}
}

func configureOTGISIS(t *testing.T, name string, dev gosnappi.Device, atePort attributes, ate *ondatra.ATEDevice, i int) {
	t.Helper()

	isisAttrs := &otgconfighelpers.ISISAttrs{
		Name:          name + strconv.Itoa(i+int(atePort.index*10)) + ".ISIS",
		SystemID:      atePort.ateISISSysID + strconv.Itoa(int(i)),
		Hostname:      name + strconv.Itoa(i+int(atePort.index*10)) + ".ISIS",
		AreaAddresses: []string{ateAreaAddress},
		Interfaces: []*otgconfighelpers.ISISInterfaceAttrs{
			{
				Name:        name + strconv.Itoa(i+int(atePort.index*10)) + ".ISISInt",
				EthName:     dev.Ethernets().Items()[0].Name(),
				NetworkType: gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT,
				LevelType:   gosnappi.IsisInterfaceLevelType.LEVEL_2,
				Metric:      10,
			},
		},
	}
	isis := otgconfighelpers.ConfigureISIS(t, dev, isisAttrs)
	loopbackV4, eMsg := atePort.ip4Loopback(i)
	if eMsg != "" {
		t.Logf("Error configuring loopbackV4: %v", eMsg)
	}
	loopbackV6, eMsg := atePort.ip6Loopback(i)
	if eMsg != "" {
		t.Logf("Error configuring loopbackV6: %v", eMsg)
	}

	otgconfighelpers.AddISISRoutesV4(isis, name+strconv.Itoa(int(i))+".ISISV4", 10, atePort.v4Route(i), 30, atePort.v4ISISRouteCount)
	otgconfighelpers.AddISISRoutesV6(isis, name+strconv.Itoa(int(i))+".ISISV6", 10, atePort.v6Route(i), 64, atePort.v6ISISRouteCount)
	otgconfighelpers.AddISISRoutesV4(isis, name+"PNH"+strconv.Itoa(int(i))+".ISISV4", 10, "60.0.0.0", 8, 1)
	otgconfighelpers.AddISISRoutesV6(isis, name+"PNH"+strconv.Itoa(int(i))+".ISISV6", 10, "2010::", 16, 1)
	otgconfighelpers.AddISISRoutesV4(isis, "ISISPort4V4"+strconv.Itoa(int(i)), 10, loopbackV4, 32, 1)
	otgconfighelpers.AddISISRoutesV6(isis, "ISISPort4V6"+strconv.Itoa(int(i)), 10, loopbackV6, 128, 1)
}
