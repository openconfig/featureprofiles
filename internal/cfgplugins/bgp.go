// Copyright 2023 Google LLC
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

package cfgplugins

import (
	"fmt"
	"math/big"
	"net"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// PortCount of topology
type PortCount int

const (
	// RPLPermitAll policy
	RPLPermitAll = "PERMIT-ALL"
	// ALLOW policy
	ALLOW = "ALLOW"

	// DutAS dut AS
	DutAS = uint32(65501)
	// AteAS1 for ATE port1
	AteAS1 = uint32(65511)
	// AteAS2 for ATE port2
	AteAS2 = uint32(65512)
	// AteAS3 for ATE port3
	AteAS3 = uint32(65513)
	// AteAS4 for ATE port4
	AteAS4 = uint32(65514)

	// BGPPeerGroup1 for ATE port1
	BGPPeerGroup1 = "BGP-PEER-GROUP1"
	// BGPPeerGroup2 for ATE port2
	BGPPeerGroup2 = "BGP-PEER-GROUP2"
	// BGPPeerGroup3 for ATE port3
	BGPPeerGroup3 = "BGP-PEER-GROUP3"
	// BGPPeerGroup4 for ATE port4
	BGPPeerGroup4 = "BGP-PEER-GROUP4"

	// PTBGP is shorthand for the long oc protocol type constant
	PTBGP        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	routeTimeout = 30 * time.Second
)

var (
	plenIPv4 = uint8(30)
	plenIPv6 = uint8(126)

	dutPort1 = &attrs.Attributes{
		Name:    "port1",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: plenIPv6,
	}
	dutPort2 = &attrs.Attributes{
		Name:    "port2",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:5",
		IPv6Len: plenIPv6,
	}
	dutPort3 = &attrs.Attributes{
		Name:    "port3",
		IPv4:    "192.0.2.9",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:9",
		IPv6Len: plenIPv6,
	}
	dutPort4 = &attrs.Attributes{
		Name:    "port4",
		IPv4:    "192.0.2.13",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:d",
		IPv6Len: plenIPv6,
	}

	atePort1 = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: plenIPv6,
	}
	atePort2 = &attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:6",
		IPv6Len: plenIPv6,
	}
	atePort3 = &attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:a",
		IPv6Len: plenIPv6,
	}
	atePort4 = &attrs.Attributes{
		Name:    "port4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:0:2:e",
		IPv6Len: plenIPv6,
	}

	bgpName = "BGP"

	// PortCount2 use this for topology of 2 ports
	PortCount2 PortCount = 2
	// PortCount4 use this for topology of 4 ports
	PortCount4 PortCount = 4
)

// BGPSession is a convenience wrapper around the dut, ate, ports, and topology we're using.
type BGPSession struct {
	DUT             *ondatra.DUTDevice
	ATE             *ondatra.ATEDevice
	OndatraDUTPorts []*ondatra.Port
	OndatraATEPorts []*ondatra.Port
	ATEIntfs        []gosnappi.Device

	DUTConf *oc.Root
	ATETop  gosnappi.Config

	DUTPorts        []*attrs.Attributes
	ATEPorts        []*attrs.Attributes
	afiTypes        []oc.E_BgpTypes_AFI_SAFI_TYPE
	networkInstance string
}

// BGPConfig holds all parameters needed to configure BGP on the DUT.
type BGPConfig struct {
	// DutAS is the AS number of the DUT.
	DutAS uint32
	// ECMPMaxPath is the maximum number of paths to advertise per prefix for both iBGP and eBGP.
	ECMPMaxPath uint32
	// RouterID is the router ID of the DUT. (Usually the IPv4 address.)
	RouterID string
	//Maximum Routes
	EnableMaxRoutes bool
	//Peer Groups
	PeerGroups []string
}

// BGPNeighborConfig holds params for creating BGP neighbors + peer groups.
type BGPNeighborConfig struct {
	AteAS            uint32
	PortName         string
	NeighborIPv4     string
	NeighborIPv6     string
	IsLag            bool
	MultiPathEnabled bool
	PolicyName       *string
}

// BgpNeighborScale holds parameters for configuring BGP neighbors in a scale test.
type BgpNeighborScale struct {
	As         uint32
	Neighborip string
	IsV4       bool
	Pg         string
}

// EBgpConfigScale holds parameters for configuring eBGP peers in a scale test.
// Use same value for AteASV4 and AteASV6 to configures ipv4 and ipv6 in the same AS.
type EBgpConfigScale struct {
	AteASV4       uint32
	AteASV6       uint32
	AtePortIPV4   string
	AtePortIPV6   string
	PeerV4GrpName string
	PeerV6GrpName string
	NumOfPeers    uint32
	PortName      string
}

// VrfBGPState holds the parameters to verify BGP neighbors state.
type VrfBGPState struct {
	NetworkInstanceName string
	NeighborIPs         []string
}

// BMPConfigParams holds the parameters to bgp BMP collector
type BMPConfigParams struct {
	DutAS        uint32
	BGPObj       *oc.NetworkInstance_Protocol_Bgp
	Source       string
	LocalAddr    string
	StationAddr  string
	StationPort  uint16
	StatsTimeOut uint16
}

// NewBGPSession creates a new BGPSession using the default global config, and
// configures the interfaces on the dut and the ate based in given topology port count.
// Only supports 2 and 4 port DUT-ATE topology
func NewBGPSession(t *testing.T, pc PortCount, ni *string) *BGPSession {
	conf := &BGPSession{
		DUT:             ondatra.DUT(t, "dut"),
		DUTConf:         &oc.Root{},
		DUTPorts:        []*attrs.Attributes{dutPort1, dutPort2},
		ATEPorts:        []*attrs.Attributes{atePort1, atePort2},
		OndatraDUTPorts: make([]*ondatra.Port, int(pc)),
		OndatraATEPorts: make([]*ondatra.Port, int(pc)),
		ATEIntfs:        make([]gosnappi.Device, int(pc)),
	}

	if pc == PortCount4 {
		conf.DUTPorts = append(conf.DUTPorts, dutPort3, dutPort4)
		conf.ATEPorts = append(conf.ATEPorts, atePort3, atePort4)
	}

	for i := 0; i < int(pc); i++ {
		conf.OndatraDUTPorts[i] = conf.DUT.Port(t, "port"+strconv.Itoa(i+1))
		conf.DUTPorts[i].ConfigOCInterface(conf.DUTConf.GetOrCreateInterface(conf.OndatraDUTPorts[i].Name()), conf.DUT)
	}

	if ate, ok := ondatra.ATEs(t)["ate"]; ok {
		conf.ATE = ate
		conf.ATETop = gosnappi.NewConfig()
		for i := 0; i < int(pc); i++ {
			conf.OndatraATEPorts[i] = conf.ATE.Port(t, "port"+strconv.Itoa(i+1))
			conf.ATEIntfs[i] = conf.ATEPorts[i].AddToOTG(conf.ATETop, conf.OndatraATEPorts[i], conf.DUTPorts[i])
		}
	}

	if ni == nil {
		fptest.ConfigureDefaultNetworkInstance(t, conf.DUT)
		conf.networkInstance = deviations.DefaultNetworkInstance(conf.DUT)
	} else {
		conf.networkInstance = *ni
	}

	return conf
}

// WithEBGP adds eBGP specific config
func (bs *BGPSession) WithEBGP(t *testing.T, afiTypes []oc.E_BgpTypes_AFI_SAFI_TYPE, bgpPorts []string, isSamePG, isSameAS bool) *BGPSession {
	for _, afiType := range afiTypes {
		if afiType != oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST && afiType != oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST {
			t.Fatalf("Unsupported AFI type: %v", afiType)
		}
	}
	bs.afiTypes = afiTypes

	asNumbers := []uint32{AteAS1, AteAS2, AteAS3, AteAS4}
	if isSameAS {
		asNumbers = []uint32{AteAS1, AteAS1, AteAS1, AteAS1}
	}

	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)
	for i, otgPort := range bs.ATEPorts {
		if !containsValue(bgpPorts, otgPort.Name) {
			continue
		}
		bgp := devices[i].Bgp().SetRouterId(otgPort.IPv4)

		for _, afiType := range afiTypes {
			switch afiType {
			case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
				ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
				bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(devices[i].Name() + ".BGP4.peer")
				bgp4Peer.SetPeerAddress(ipv4.Gateway())
				bgp4Peer.SetAsNumber(asNumbers[i])
				bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
				bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
			case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
				ipv6 := devices[i].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
				bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(devices[i].Name() + ".BGP6.peer")
				bgp6Peer.SetPeerAddress(ipv6.Gateway())
				bgp6Peer.SetAsNumber(asNumbers[i])
				bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
				bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
			}
		}
	}

	niProtocol := bs.DUTConf.GetOrCreateNetworkInstance(bs.networkInstance).GetOrCreateProtocol(PTBGP, bgpName)
	neighborConfig := bs.buildNeigborConfig(isSamePG, isSameAS, bgpPorts)
	niProtocol.Bgp = BuildBGPOCConfig(t, bs.DUT, dutPort1.IPv4, afiTypes, neighborConfig)

	err := bs.configureRoutingPolicy()
	if err != nil {
		t.Fatalf("Failed to configure routing policy: %v", err)
	}

	return bs
}

func (bs *BGPSession) configureRoutingPolicy() error {
	rp := bs.DUTConf.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(RPLPermitAll)
	stmt, err := pdef.AppendNewStatement("20")
	if err != nil {
		return err
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return nil
}

// PushAndStart calls PushDUT and PushAndStartATE to send config to both devices
func (bs *BGPSession) PushAndStart(t testing.TB) error {
	t.Helper()
	if err := bs.PushDUT(t); err != nil {
		return err
	}
	bs.PushAndStartATE(t)
	return nil
}

// PushDUT replaces DUT config with s.dutConf. Only interfaces and the ISIS protocol are written
func (bs *BGPSession) PushDUT(t testing.TB) error {
	fptest.WriteQuery(t, "Updating Config", gnmi.OC().Config(), bs.DUTConf)
	res := gnmi.Update(t, bs.DUT, gnmi.OC().Config(), bs.DUTConf)
	if res == nil {
		t.Fatal("Failed to set DUT config: gnmi.Update returned nil result; check `Updating Config` for update request")
	}

	if deviations.ExplicitInterfaceInDefaultVRF(bs.DUT) {
		for i := 0; i < len(bs.DUTPorts); i++ {
			fptest.AssignToNetworkInstance(t, bs.DUT, bs.OndatraDUTPorts[i].Name(), bs.networkInstance, 0)
		}
	}
	if deviations.ExplicitPortSpeed(bs.DUT) {
		for i := 0; i < len(bs.DUTPorts); i++ {
			fptest.SetPortSpeed(t, bs.OndatraDUTPorts[i])
		}
	}
	return nil
}

// PushAndStartATE pushes the ATETop to the ATE and starts protocols on it.
func (bs *BGPSession) PushAndStartATE(t testing.TB) {
	t.Helper()
	otg := bs.ATE.OTG()
	otg.PushConfig(t, bs.ATETop)
	otg.StartProtocols(t)

	for _, afiType := range bs.afiTypes {
		switch afiType {
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
			otgutils.WaitForARP(t.(*testing.T), otg, bs.ATETop, "IPv4")
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
			otgutils.WaitForARP(t.(*testing.T), otg, bs.ATETop, "IPv6")
		}
	}
}

// VerifyDUTBGPEstablished verifies on DUT BGP peer establishment
func VerifyDUTBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, duration ...time.Duration) {
	var timeout time.Duration
	if len(duration) > 0 {
		timeout = duration[0]
	} else {
		timeout = 2 * time.Minute
	}
	dni := deviations.DefaultNetworkInstance(dut)
	nSessionState := gnmi.OC().NetworkInstance(dni).Protocol(PTBGP, bgpName).Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, dut, nSessionState, timeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("DUT BGP sessions established")
}

// VerifyOTGBGPEstablished verifies on OTG BGP peer establishment
func VerifyOTGBGPEstablished(t *testing.T, ate *ondatra.ATEDevice, duration ...time.Duration) {
	var timeout time.Duration
	if len(duration) > 0 {
		timeout = duration[0]
	} else {
		timeout = 2 * time.Minute
	}
	pSessionState := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, ate.OTG(), pSessionState, timeout, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("OTG BGP sessions established")
}

// NeighborConfig to  hold neighbor specific config
type NeighborConfig struct {
	Name         string
	IPv4Neighbor string
	IPv6Neighbor string
	PeerGroup    string
	AS           uint32
}

// BgpNeighbor holds BGP Peer information.
type BgpNeighbor struct {
	LocalAS    uint32
	PeerAS     uint32
	Neighborip string
	IsV4       bool
	PeerGrp    string
}

// buildNeigborConfig builds neighbor config based on given flags
func (bs *BGPSession) buildNeigborConfig(isSamePG, isSameAS bool, bgpPorts []string) []*NeighborConfig {
	nc1 := &NeighborConfig{
		Name:         "port1",
		IPv4Neighbor: atePort1.IPv4,
		IPv6Neighbor: atePort1.IPv6,
		PeerGroup:    BGPPeerGroup1,
		AS:           AteAS1,
	}
	nc2 := &NeighborConfig{
		Name:         "port2",
		IPv4Neighbor: atePort2.IPv4,
		IPv6Neighbor: atePort2.IPv6,
		PeerGroup:    BGPPeerGroup2,
		AS:           AteAS2,
	}
	nc3 := &NeighborConfig{
		Name:         "port3",
		IPv4Neighbor: atePort3.IPv4,
		IPv6Neighbor: atePort3.IPv6,
		PeerGroup:    BGPPeerGroup3,
		AS:           AteAS3,
	}
	nc4 := &NeighborConfig{
		Name:         "port4",
		IPv4Neighbor: atePort4.IPv4,
		IPv6Neighbor: atePort4.IPv6,
		PeerGroup:    BGPPeerGroup4,
		AS:           AteAS4,
	}
	ncAll := []*NeighborConfig{nc1, nc2, nc3, nc4}

	var validNC []*NeighborConfig
	for _, nc := range ncAll[:len(bs.DUTPorts)] {
		if containsValue(bgpPorts, nc.Name) {
			validNC = append(validNC, nc)
		}
	}

	if isSamePG {
		for _, nc := range validNC {
			nc.PeerGroup = BGPPeerGroup1
		}
	}
	if isSameAS {
		for _, nc := range validNC {
			nc.AS = AteAS1
		}
	}

	return validNC
}

// BuildBGPOCConfig builds the BGP OC config applying global, neighbors and peer-group config
func BuildBGPOCConfig(t *testing.T, dut *ondatra.DUTDevice, routerID string, afiTypes []oc.E_BgpTypes_AFI_SAFI_TYPE, neighborConfig []*NeighborConfig) *oc.NetworkInstance_Protocol_Bgp {
	afiSafiGlobal := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{}
	for _, afiType := range afiTypes {
		afiSafiGlobal[afiType] = &oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
			AfiSafiName: afiType,
			Enabled:     ygot.Bool(true),
		}
	}

	global := &oc.NetworkInstance_Protocol_Bgp_Global{
		As:       ygot.Uint32(DutAS),
		RouterId: ygot.String(routerID),
		AfiSafi:  afiSafiGlobal,
	}

	neighbors := make(map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor)
	peerGroups := make(map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup)
	var neighbor string
	for _, nc := range neighborConfig {
		for _, afiType := range afiTypes {
			switch afiType {
			case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
				neighbor = nc.IPv4Neighbor
			case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
				neighbor = nc.IPv6Neighbor
			default:
				t.Fatalf("Unsupported AFI type: %v", afiType)
			}

			neighbors[neighbor] = &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(nc.AS),
				PeerGroup:       ygot.String(nc.PeerGroup),
				NeighborAddress: ygot.String(neighbor),
				AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
					afiType: {
						AfiSafiName: afiType,
						Enabled:     ygot.Bool(true),
					},
				},
			}

			peerGroups[nc.PeerGroup] = getPeerGroup(nc.PeerGroup, dut, afiTypes)
		}
	}

	return &oc.NetworkInstance_Protocol_Bgp{
		Global:    global,
		Neighbor:  neighbors,
		PeerGroup: peerGroups,
	}
}

// getPeerGroup build peer-config
func getPeerGroup(pgn string, dut *ondatra.DUTDevice, afiType []oc.E_BgpTypes_AFI_SAFI_TYPE) *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	pg := bgp.GetOrCreatePeerGroup(pgn)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		// policy under peer group
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{RPLPermitAll})
		rpl.SetImportPolicy([]string{RPLPermitAll})
		return pg
	}

	// policy under peer group AFI
	for _, afi := range afiType {
		afisafi := pg.GetOrCreateAfiSafi(afi)
		afisafi.Enabled = ygot.Bool(true)
		rpl := afisafi.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{RPLPermitAll})
		rpl.SetImportPolicy([]string{RPLPermitAll})
	}
	return pg
}

func containsValue[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// BGPClearConfig removes all BGP configuration from the DUT.
func BGPClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll[*oc.NetworkInstance_Table](t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

// VerifyBGPCapabilities function is used to Verify BGP capabilities like route refresh as32 and mpbgp.
func VerifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice, nbrs []*BgpNeighbor) {
	t.Log("Verifying BGP capabilities")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrs {
		nbrPath := statePath.Neighbor(nbr.Neighborip)

		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_ASN32:         false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
		}
		for _, c := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[c] = true
		}
		for c, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", c)
			}
		}
	}
}

// VerifyPortsUp asserts that each port on the device is operating.
func VerifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// DeviationAristaBGPNeighborMaxPrefixes updates the max-prefixes of a specific BGP neighbor.
// This is an Arista specific augmented model which sets the following path:
// /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/prefix-limit/config/max-prefixes
// Set max-prefixes to 0 will mean no limit will be set.
// Tracking the removal of this deviation in b/438620249
func DeviationAristaBGPNeighborMaxPrefixes(t *testing.T, dut *ondatra.DUTDevice, neighborIP string, maxPrefixes uint32) {
	gpbSetRequest := &gnmipb.SetRequest{
		Update: []*gnmipb.Update{{
			Path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "network-instances"},
					{Name: "network-instance", Key: map[string]string{"name": deviations.DefaultNetworkInstance(dut)}},
					{Name: "protocols"},
					{Name: "protocol", Key: map[string]string{"name": "BGP", "identifier": "BGP"}},
					{Name: "bgp"},
					{Name: "neighbors"},
					{Name: "neighbor", Key: map[string]string{"neighbor-address": neighborIP}},
					{Name: "prefix-limit"},
					{Name: "config"},
					{Name: "max-prefixes"},
				},
			},
			Val: &gnmipb.TypedValue{
				Value: &gnmipb.TypedValue_UintVal{
					UintVal: uint64(maxPrefixes),
				},
			},
		}},
	}
	gnmiClient := dut.RawAPIs().GNMI(t)
	if _, err := gnmiClient.Set(t.Context(), gpbSetRequest); err != nil {
		t.Errorf("Unexpected error max-prefix: %v", err)
	}
}

// handleMaxPrefixesDeviation updates neighbor max prefixes only if deviation BGPMissingOCMaxPrefixesConfiguration
// is set. TODO: Add config created in DeviationAristaBGPNeighborMaxPrefixes to the SetBatch.
func handleMaxPrefixesDeviation(t *testing.T, dut *ondatra.DUTDevice, _ *gnmi.SetBatch, cfg BGPNeighborsConfig) error {
	t.Helper()
	if !deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		return nil
	}
	switch dut.Vendor() {
	case ondatra.ARISTA:
		for _, nbr := range cfg.Nbrs {
			DeviationAristaBGPNeighborMaxPrefixes(t, dut, nbr.Neighborip, 0)
		}
	default:
		return fmt.Errorf("deviation not expected for vendor %v", dut.Vendor())
	}
	return nil
}

// sameAS checks if all neighbors have the same local and peer AS.
func sameAS(nbrs []*BgpNeighbor) bool {
	for _, nbr := range nbrs {
		if nbr.LocalAS != nbrs[0].LocalAS {
			return false
		}
		if nbr.PeerAS != nbrs[0].PeerAS {
			return false
		}
	}
	return true
}

// handleMultipathDeviation implements the deviation logic whether multipath config
// at the afisafi level is supported or not. It updates the root object with the
// necessary configuration.
func handleMultipathDeviation(t *testing.T, dut *ondatra.DUTDevice, root *oc.Root, cfg BGPNeighborsConfig) error {
	t.Helper()
	bgp := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	// Handle MultipathUnderAfiSafi deviation and Configure Multipath for Cisco
	if deviations.EnableMultipathUnderAfiSafi(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			global := bgp.GetOrCreateGlobal()
			// set the maxpaths as 2 as we can expect max of 2 paths in the test.
			global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().MaximumPaths = ygot.Uint32(2)
			global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().MaximumPaths = ygot.Uint32(2)
			return nil
		default:
			return fmt.Errorf("deviation not expected for vendor %v", dut.Vendor())
		}
	}
	// Handle MultipathUnsupportedNeighborOrAfisafi deviation and Configure Multipath for Juniper
	if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
		switch dut.Vendor() {
		case ondatra.JUNIPER:
			bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV4).GetOrCreateUseMultiplePaths().
				SetEnabled(true)
			bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV6).GetOrCreateUseMultiplePaths().
				SetEnabled(true)
			return nil
		default:
			return fmt.Errorf("deviation not expected for vendor %v", dut.Vendor())
		}
	}

	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		GetOrCreateUseMultiplePaths().
		GetOrCreateEbgp().
		SetMaximumPaths(2)
	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
		GetOrCreateUseMultiplePaths().
		GetOrCreateEbgp().
		SetMaximumPaths(2)
	bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		GetOrCreateUseMultiplePaths().
		SetEnabled(true)
	bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
		GetOrCreateUseMultiplePaths().
		SetEnabled(true)
	return nil
}

// BGPNeighborsConfig contains the configuration for configuring multiple BGP neighbors.
type BGPNeighborsConfig struct {
	// Router ID of the BGP neighbors.
	RouterID string
	// Name of the peer group for IPv4 neighbors.
	PeerGrpNameV4 string
	// Name of the peer group for IPv6 neighbors.
	PeerGrpNameV6 string
	// List of BGP neighbors to be configured.
	Nbrs []*BgpNeighbor
}

// CreateBGPNeighbors creates BGP neighbors for the given router ID, peer group names, and
// neighbors. The global AS and router ID are set to the AS and router ID of the first neighbor,
// assuming that all neighbors provided have the same local AS and the same peer AS.
// The sb is updated with the BGP neighbors configuration.
func CreateBGPNeighbors(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg BGPNeighborsConfig) error {
	if len(cfg.Nbrs) == 0 {
		t.Logf("No BGP neighbors found for router ID: %s, peer group name v4: %s, peer group name v6: %s", cfg.RouterID, cfg.PeerGrpNameV4, cfg.PeerGrpNameV6)
		return nil
	}
	if !sameAS(cfg.Nbrs) {
		return fmt.Errorf("BGP neighbors have different AS numbers: %v", cfg.Nbrs)
	}
	peerAS := cfg.Nbrs[0].PeerAS

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	protocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := protocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(cfg.Nbrs[0].LocalAS)
	global.SetRouterId(cfg.RouterID)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		SetEnabled(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
		SetEnabled(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgV4 := bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV4)
	pgV4.SetPeerAs(peerAS)
	pgV4AFI := pgV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgV4AFI.SetEnabled(true)
	applyPolicyV4 := pgV4AFI.GetOrCreateApplyPolicy()
	applyPolicyV4.SetImportPolicy([]string{ALLOW})
	applyPolicyV4.SetExportPolicy([]string{ALLOW})

	pgV6 := bgp.GetOrCreatePeerGroup(cfg.PeerGrpNameV6)
	pgV6.SetPeerAs(peerAS)
	pgV6AFI := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgV6AFI.SetEnabled(true)
	applyPolicyV6 := pgV6AFI.GetOrCreateApplyPolicy()
	applyPolicyV6.SetImportPolicy([]string{ALLOW})
	applyPolicyV6.SetExportPolicy([]string{ALLOW})

	if err := handleMultipathDeviation(t, dut, root, cfg); err != nil {
		return err
	}

	for _, nbr := range cfg.Nbrs {
		neighbor := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		neighbor.SetPeerAs(peerAS)
		neighbor.SetEnabled(true)
		switch {
		case nbr.IsV4:
			neighbor.
				SetPeerGroup(cfg.PeerGrpNameV4)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
				SetEnabled(true)
		default:
			neighbor.
				SetPeerGroup(cfg.PeerGrpNameV6)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
				SetEnabled(true)
		}
	}
	if err := handleMaxPrefixesDeviation(t, dut, sb, cfg); err != nil {
		return err
	}
	gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), root.GetNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP"))
	return nil
}

// ConfigureBGPNeighbor configures a BGP neighbor.
func ConfigureBGPNeighbor(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance, routerID, peerAddress string, routerAS, peerAS uint32, ipType string, sendReceivePaths bool) {
	if ni == nil {
		t.Fatalf("Network Instance is not configured")
	}
	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(routerAS)
	global.RouterId = ygot.String(routerID)

	neighbor := bgp.GetOrCreateNeighbor(peerAddress)
	neighbor.PeerAs = ygot.Uint32(peerAS)
	neighbor.Enabled = ygot.Bool(true)
	neighbor.SendCommunityType = []oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_NONE}

	neighbor.GetOrCreateApplyPolicy().DefaultExportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	neighbor.GetOrCreateApplyPolicy().DefaultImportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE

	var nAfiSafi *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi
	switch ipType {
	case IPv4:
		nAfiSafi = neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		nAfiSafi.GetOrCreateIpv4Unicast().SendDefaultRoute = ygot.Bool(true)
	case IPv6:
		nAfiSafi = neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		nAfiSafi.GetOrCreateIpv6Unicast().SendDefaultRoute = ygot.Bool(true)
	}
	nAfiSafi.Enabled = ygot.Bool(true)
	nAfiSafi.GetOrCreateAddPaths().Receive = ygot.Bool(sendReceivePaths)
	nAfiSafi.GetOrCreateAddPaths().Send = ygot.Bool(sendReceivePaths)
}

// ConfigureDUTBGP configures BGP on the DUT using OpenConfig.
func ConfigureDUTBGP(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg BGPConfig) *oc.NetworkInstance_Protocol {
	t.Helper()
	d := gnmi.OC()

	dutBgpConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	// Create BGP config
	dutBgpConf := &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, Name: ygot.String("BGP"), Bgp: &oc.NetworkInstance_Protocol_Bgp{}}
	bgp := dutBgpConf.Bgp
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(cfg.DutAS)
	global.RouterId = ygot.String(cfg.RouterID)

	af4 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	if cfg.EnableMaxRoutes {
		bgpMaxRouteCfg := new(strings.Builder)
		fmt.Fprintf(bgpMaxRouteCfg, "router bgp %d\n", cfg.DutAS)

		for _, pg := range cfg.PeerGroups {
			fmt.Fprintf(bgpMaxRouteCfg, "neighbor %s maximum-routes 0\n", pg)
		}

		helpers.GnmiCLIConfig(t, dut, bgpMaxRouteCfg.String())
	}

	// Handle multipath deviation
	if cfg.ECMPMaxPath > 0 {
		if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
			t.Log("Executing CLI commands for multipath deviation")
			bgpRouteConfig := fmt.Sprintf(`
		router bgp %d
		address-family ipv4
		maximum-paths %[2]d ecmp %[2]d
		bgp bestpath as-path multipath-relax
		address-family ipv6
		maximum-paths %[2]d ecmp %[2]d
		bgp bestpath as-path multipath-relax
		`, cfg.DutAS, cfg.ECMPMaxPath)
			helpers.GnmiCLIConfig(t, dut, bgpRouteConfig)
		} else {
			// TODO: Once multipath is fully supported via OpenConfig across all platforms,
			// remove CLI fallback and rely solely on OC configuration.
			v4Multipath := af4.GetOrCreateUseMultiplePaths()
			v4Multipath.SetEnabled(true)
			v4Multipath.GetOrCreateIbgp().SetMaximumPaths(cfg.ECMPMaxPath)
			v4Multipath.GetOrCreateEbgp().SetMaximumPaths(cfg.ECMPMaxPath)

			v6Multipath := af6.GetOrCreateUseMultiplePaths()
			v6Multipath.SetEnabled(true)
			v6Multipath.GetOrCreateIbgp().SetMaximumPaths(cfg.ECMPMaxPath)
			v6Multipath.GetOrCreateEbgp().SetMaximumPaths(cfg.ECMPMaxPath)

			if !deviations.SkipSettingAllowMultipleAS(dut) {
				v4Multipath.GetOrCreateEbgp().SetAllowMultipleAs(true)
				v6Multipath.GetOrCreateEbgp().SetAllowMultipleAs(true)
			}
		}
	}
	gnmi.BatchUpdate(batch, dutBgpConfPath.Config(), dutBgpConf)
	return dutBgpConf
}

// AppendBGPNeighbor configures BGP peer-groups and neighbors into a batch.
func AppendBGPNeighbor(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, bgp *oc.NetworkInstance_Protocol_Bgp, cfg BGPNeighborConfig) *oc.NetworkInstance_Protocol_Bgp {
	t.Helper()
	// === Peer Group for IPv4 ===
	pgv4Name := cfg.PortName + "BGP-PEER-GROUP-V4"
	pgv4 := bgp.GetOrCreatePeerGroup(pgv4Name)
	pgv4.PeerAs = ygot.Uint32(cfg.AteAS)
	pgv4.PeerGroupName = ygot.String(pgv4Name)
	pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgafv4.Enabled = ygot.Bool(true)
	rpl4 := pgafv4.GetOrCreateApplyPolicy()
	if cfg.PolicyName != nil {
		rpl4.ImportPolicy = []string{*cfg.PolicyName}
		rpl4.ExportPolicy = []string{*cfg.PolicyName}
	} else {
		rpl4.ImportPolicy = []string{ALLOW}
		rpl4.ExportPolicy = []string{ALLOW}
	}

	// === Peer Group for IPv6 ===
	pgv6Name := cfg.PortName + "BGP-PEER-GROUP-V6"
	pgv6 := bgp.GetOrCreatePeerGroup(pgv6Name)
	pgv6.PeerAs = ygot.Uint32(cfg.AteAS)
	pgv6.PeerGroupName = ygot.String(pgv6Name)
	pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgafv6.Enabled = ygot.Bool(true)
	rpl6 := pgafv6.GetOrCreateApplyPolicy()
	if cfg.PolicyName != nil {
		rpl6.ImportPolicy = []string{*cfg.PolicyName}
		rpl6.ExportPolicy = []string{*cfg.PolicyName}
	} else {
		rpl6.ImportPolicy = []string{ALLOW}
		rpl6.ExportPolicy = []string{ALLOW}
	}

	if cfg.MultiPathEnabled {
		if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
			pgv4.GetOrCreateUseMultiplePaths().SetEnabled(true)
			pgv6.GetOrCreateUseMultiplePaths().SetEnabled(true)
		} else {
			pgafv4.GetOrCreateUseMultiplePaths().SetEnabled(true)
			pgafv6.GetOrCreateUseMultiplePaths().SetEnabled(true)
		}
	}

	// === IPv4 Neighbor ===
	nv4 := bgp.GetOrCreateNeighbor(cfg.NeighborIPv4)
	nv4.PeerAs = ygot.Uint32(cfg.AteAS)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(pgv4Name)
	afisafi4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afisafi4.Enabled = ygot.Bool(true)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)

	// === IPv6 Neighbor ===
	nv6 := bgp.GetOrCreateNeighbor(cfg.NeighborIPv6)
	nv6.PeerAs = ygot.Uint32(cfg.AteAS)
	nv6.Enabled = ygot.Bool(true)
	nv6.PeerGroup = ygot.String(pgv6Name)
	afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afisafi6.Enabled = ygot.Bool(true)
	nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)

	// Enable multihop on LAG neighbors
	if cfg.IsLag {
		nv4.GetOrCreateEbgpMultihop().SetMultihopTtl(5)
		nv6.GetOrCreateEbgpMultihop().SetMultihopTtl(5)
	}
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Config(), bgp)

	return bgp
}

// BgpISISRedistribution configures the BGP to ISIS redistribution for a given AFI/SAFI.
func BgpISISRedistribution(t *testing.T, dut *ondatra.DUTDevice, afisafi string, b *gnmi.SetBatch, importPolicy string) *oc.Root {
	t.Helper()
	d := &oc.Root{}
	dni := deviations.DefaultNetworkInstance(dut)

	if deviations.EnableTableConnections(dut) {
		fptest.ConfigEnableTbNative(t, dut)
	}
	var tableConn *oc.NetworkInstance_TableConnection
	if afisafi == "ipv4" {
		tableConn = d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4)
	} else if afisafi == "ipv6" {
		tableConn = d.GetOrCreateNetworkInstance(dni).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6)
	}

	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tableConn.SetDisableMetricPropagation(false)
	}
	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		tableConn.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	}

	if importPolicy != "" {
		tableConn.SetImportPolicy([]string{importPolicy})
	}

	if afisafi == "ipv4" {
		gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(dni).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConn)
	} else if afisafi == "ipv6" {
		gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(dni).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, oc.Types_ADDRESS_FAMILY_IPV6).Config(), tableConn)
	}
	return d
}

// GlobalOption is a function that sets options for BGP Global configuration.
type GlobalOption func(*oc.NetworkInstance_Protocol_Bgp_Global, *ondatra.DUTDevice)

// WithAS sets the global Autonomous System number.
func WithAS(as uint32) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, _ *ondatra.DUTDevice) {
		g.As = ygot.Uint32(as)
	}
}

// WithRouterID sets the BGP Router ID.
func WithRouterID(id string) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, _ *ondatra.DUTDevice) {
		if id != "" {
			g.RouterId = ygot.String(id)
		}
	}
}

// WithGlobalGracefulRestart configures global BGP Graceful Restart settings.
func WithGlobalGracefulRestart(enabled bool, restartTime, staleTime uint16) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, _ *ondatra.DUTDevice) {
		bgpGR := g.GetOrCreateGracefulRestart()
		bgpGR.Enabled = ygot.Bool(enabled)
		if enabled {
			if restartTime > 0 {
				bgpGR.SetRestartTime(restartTime)
			}
			if staleTime > 0 {
				bgpGR.SetStaleRoutesTime(staleTime)
			}
		}
	}
}

// WithGlobalAfiSafiEnabled enables or disables a global AFI/SAFI.
func WithGlobalAfiSafiEnabled(afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE, enabled bool) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, _ *ondatra.DUTDevice) {
		g.GetOrCreateAfiSafi(afiSafi).Enabled = ygot.Bool(enabled)
	}
}

// WithExternalRouteDistance sets the default external route distance.
func WithExternalRouteDistance(distance uint8) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, dut *ondatra.DUTDevice) {
		if !deviations.BgpDistanceOcPathUnsupported(dut) {
			g.GetOrCreateDefaultRouteDistance().ExternalRouteDistance = ygot.Uint8(distance)
		}
	}
}

// WithGlobalEBGPMultipath configures global EBGP multipath settings.
func WithGlobalEBGPMultipath(maxPaths uint32) GlobalOption {
	return func(g *oc.NetworkInstance_Protocol_Bgp_Global, dut *ondatra.DUTDevice) {
		// Global AllowMultipleAs based on deviation
		if deviations.SkipSettingAllowMultipleAS(dut) {
			g.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetAllowMultipleAs(true)
		}

		// Configure MaximumPaths
		if deviations.EnableMultipathUnderAfiSafi(dut) {
			g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().MaximumPaths = ygot.Uint32(maxPaths)
			g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp().MaximumPaths = ygot.Uint32(maxPaths)
		} else {
			g.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().MaximumPaths = ygot.Uint32(maxPaths)
		}

		// Additional Global AFI/SAFI EBGP multipath settings for IPv4/v6
		if !deviations.SkipAfiSafiPathForBgpMultipleAs(dut) {
			gEBGPAfiV4 := g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp()
			gEBGPAfiV6 := g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateEbgp()

			if deviations.SkipSettingAllowMultipleAS(dut) {
				gEBGPAfiV4.AllowMultipleAs = ygot.Bool(true)
				gEBGPAfiV6.AllowMultipleAs = ygot.Bool(true)
			}
			// This ensures MaximumPaths is set on the AFI/SAFI level
			gEBGPAfiV4.MaximumPaths = ygot.Uint32(maxPaths)
			gEBGPAfiV6.MaximumPaths = ygot.Uint32(maxPaths)
		} else {
			fmt.Printf("SkipAfiSafiPathForBgpMultipleAs is true, skipping additional Global AFI/SAFI path config for IPv4.")
		}
	}
}

// ConfigureGlobal applies a series of GlobalOptions to the BGP global configuration.
func ConfigureGlobal(bgp *oc.NetworkInstance_Protocol_Bgp, dut *ondatra.DUTDevice, opts ...GlobalOption) {
	if bgp == nil {
		return
	}
	g := bgp.GetOrCreateGlobal()
	for _, opt := range opts {
		opt(g, dut)
	}
}

// PeerOption is a function that sets options for BGP Peer configuration.
type PeerOption func(*oc.NetworkInstance_Protocol_Bgp_Neighbor, *ondatra.DUTDevice)

// WithPeerGroup sets the Peer Group for the neighbor.
func WithPeerGroup(pgName string, as uint32, importPolicy, exportPolicy string, addDeleteLinkBW bool) PeerOption {
	return func(n *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice) {
		n.PeerGroup = ygot.String(pgName)
		n.PeerAs = ygot.Uint32(as)
		n.Enabled = ygot.Bool(true)
	}
}

// WithPeerAfiSafiEnabled enables or disables an AFI/SAFI for the peer group.
func WithPeerAfiSafiEnabled(isV4 bool, importPolicy, exportPolicy string, addDeleteLinkBW bool) PeerOption {
	return func(n *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice) {
		var af *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi
		if isV4 {
			af = n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af.Enabled = ygot.Bool(true)
			n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
		} else {
			af = n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af.Enabled = ygot.Bool(true)
			n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
		}
		if !deviations.RoutePolicyUnderAFIUnsupported(dut) {
			rpl := af.GetOrCreateApplyPolicy()
			if importPolicy != "" {
				rpl.SetImportPolicy([]string{importPolicy})
			}
			if exportPolicy != "" {
				exportPolicies := []string{exportPolicy}
				if addDeleteLinkBW {
					exportPolicies = append([]string{"delete_linkbw"}, exportPolicies...)
				}
				rpl.SetExportPolicy(exportPolicies)
			}
		}
	}
}

// ApplyPeerPerAfiSafiRoutingPolicy applies routing policies to the peer per AFI/SAFI.
func ApplyPeerPerAfiSafiRoutingPolicy(isV4 bool, importPolicy, exportPolicy string, addDeleteLinkBW bool) PeerOption {
	return func(peer *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice) {
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			rpl := peer.GetOrCreateApplyPolicy()
			if importPolicy != "" {
				rpl.SetImportPolicy([]string{importPolicy})
			}
			if exportPolicy != "" {
				exportPolicies := []string{exportPolicy}
				if addDeleteLinkBW {
					exportPolicies = append([]string{"delete_linkbw"}, exportPolicies...)
				}
				rpl.SetExportPolicy(exportPolicies)
			}
		}
	}
}

// ConfigurePeer applies a series of PeerOptions to a BGP Peer.
func ConfigurePeer(peer *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice, opts ...PeerOption) {
	if peer == nil {
		return
	}
	for _, opt := range opts {
		opt(peer, dut)
	}
}

// PeerGroupOption is a function that sets options for BGP Peer Group configuration.
type PeerGroupOption func(*oc.NetworkInstance_Protocol_Bgp_PeerGroup, *ondatra.DUTDevice)

// WithPeerAS sets the Peer Autonomous System number for the group.
func WithPeerAS(as uint32) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, _ *ondatra.DUTDevice) {
		pg.PeerAs = ygot.Uint32(as)
	}
}

// WithPGTimers configures BGP timers for the peer group.
func WithPGTimers(holdTime, keepalive, minAdvInterval uint16) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, _ *ondatra.DUTDevice) {
		timers := pg.GetOrCreateTimers()
		if holdTime > 0 {
			timers.HoldTime = ygot.Uint16(holdTime)
		}
		if keepalive > 0 {
			timers.KeepaliveInterval = ygot.Uint16(keepalive)
		}
		if minAdvInterval > 0 {
			timers.SetMinimumAdvertisementInterval(minAdvInterval)
		}
	}
}

// WithPGDescription sets the description for the peer group.
func WithPGDescription(desc string) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, _ *ondatra.DUTDevice) {
		pg.SetDescription(desc)
	}
}

// WithPGGracefulRestart configures Graceful Restart for the peer group.
func WithPGGracefulRestart(enabled bool) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		pgGR := pg.GetOrCreateGracefulRestart()
		pgGR.Enabled = ygot.Bool(enabled)
		if enabled && !deviations.BgpGrHelperDisableUnsupported(dut) {
			pgGR.HelperOnly = ygot.Bool(false)
		}
	}
}

// WithPGSendCommunity sets the community types to send.
func WithPGSendCommunity(communities []oc.E_Bgp_CommunityType) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		if !deviations.SkipBgpSendCommunityType(dut) {
			pg.SetSendCommunityType(communities)
		}
	}
}

// WithPGAfiSafiEnabled enables or disables an AFI/SAFI for the peer group.
func WithPGAfiSafiEnabled(afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE, enabled bool, configureGR bool) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		pgaf := pg.GetOrCreateAfiSafi(afiSafi)
		pgaf.Enabled = ygot.Bool(enabled)
		if enabled && configureGR && !deviations.BgpGracefulRestartUnderAfiSafiUnsupported(dut) {
			pgaf.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
		}
	}
}

// WithPGTransport sets the transport address for the peer group.
func WithPGTransport(transportAddress string) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		if transportAddress == "" {
			// Do not apply the setting if the address is empty.
			// Only for IBGP peers
			return
		}
		pg.GetOrCreateTransport().LocalAddress = ygot.String(transportAddress)
	}
}

// WithPGMultipath configures multipath settings for the peer group.
func WithPGMultipath(pgName string, enableMultipath bool) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		// skip multipath for IBGP peers
		if !enableMultipath {
			return
		}
		var pgaf *oc.NetworkInstance_Protocol_Bgp_PeerGroup_AfiSafi
		if !deviations.SkipBgpSendCommunityType(dut) {
			pg.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
		}

		// Peer Group Multipath vendor specifics
		if strings.Contains(pgName, "V4") {
			pgaf = pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		} else {
			pgaf = pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		}

		switch dut.Vendor() {
		case ondatra.NOKIA:
			// BGP multipath enable/disable at the peer-group level not required b/376799583
			fmt.Printf("PeerGroup %s: BGP Multipath enable/disable not required under Peer-group by %s hence skipping", pgName, dut.Vendor())
		case ondatra.JUNIPER:
			pgaf.GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
			pg.GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
			pg.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetAllowMultipleAs(true)
		default:
			pgaf.GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
		}
	}
}

// ApplyPGRoutingPolicy applies routing policies to the peer group.
func ApplyPGRoutingPolicy(importPolicy, exportPolicy string, addDeleteLinkBW bool) PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			rpl := pg.GetOrCreateApplyPolicy()
			if importPolicy != "" {
				rpl.SetImportPolicy([]string{importPolicy})
			}
			if exportPolicy != "" {
				exportPolicies := []string{exportPolicy}
				if addDeleteLinkBW {
					exportPolicies = append([]string{"delete_linkbw"}, exportPolicies...)
				}
				rpl.SetExportPolicy(exportPolicies)
			}
		}
	}
}

// ConfigurePeerGroup applies a series of PeerGroupOptions to a BGP Peer Group.
func ConfigurePeerGroup(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice, opts ...PeerGroupOption) {
	if pg == nil {
		return
	}
	for _, opt := range opts {
		opt(pg, dut)
	}
}

// addBGPNeighborTCPMSSOps adds gNMI operations for BGP neighbor TCP MSS to the batch.
func addBGPNeighborTCPMSSOps(t *testing.T, b *gnmi.SetBatch, dut *ondatra.DUTDevice, nbrList []string, isDelete bool, mss uint16) {
	// TODO: will investigate if we need to add a deviation for Arista.
	if dut.Vendor() == ondatra.ARISTA {
		t.Logf("TCP MSS: dut.Vendor() == ondatra.ARISTA, PMTU discovery is enabled by default, skipping explicit TCP MSS operations.")
		return // No explicit TCP MSS operations for Arista.
	}

	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	bgpPath := ni.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrList {
		tcpMssPath := bgpPath.Neighbor(nbr).Transport().TcpMss().Config()
		if isDelete {
			gnmi.BatchDelete(b, tcpMssPath)
		} else {
			gnmi.BatchReplace(b, tcpMssPath, mss)
		}
	}
}

// ConfigureDUTBGPMaxSegmentSize configures the DUT interface MTU and BGP neighbor TCP MSS.
// intfName is the name of the interface to configure MTU on.
func ConfigureDUTBGPMaxSegmentSize(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16, nbrList []string, mss uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	isDelete := false

	t.Logf("Configuring MTU %d on interface: %s", mtu, intfName)
	AddInterfaceMTUOps(b, dut, intfName, mtu, isDelete)

	t.Logf("Configuring DUT BGP TCP-MSS for relevant neighbors")
	addBGPNeighborTCPMSSOps(t, b, dut, nbrList, isDelete, mss)

	b.Set(t, dut)
}

// DeleteDUTBGPMaxSegmentSize removes the DUT interface MTU and BGP neighbor TCP MSS configurations.
// intfName is the name of the interface to remove MTU config from.
func DeleteDUTBGPMaxSegmentSize(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16, nbrList []string, mss uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	isDelete := true

	t.Logf("Deleting MTU config on interface: %s", intfName)
	AddInterfaceMTUOps(b, dut, intfName, mtu, isDelete) // MTU value not used for delete

	t.Logf("Deleting DUT BGP TCP-MSS config for relevant neighbors")
	addBGPNeighborTCPMSSOps(t, b, dut, nbrList, isDelete, mss)

	b.Set(t, dut)
}

// BuildIPv4v6NbrScale generates a list of BgpNeighborScale configurations for IPv4 and IPv6 peers.
func BuildIPv4v6NbrScale(t *testing.T, cfg *EBgpConfigScale) []*BgpNeighborScale {
	var nbrList []*BgpNeighborScale
	asn := cfg.AteASV4
	asn6 := cfg.AteASV6
	for i := uint32(1); i <= cfg.NumOfPeers; i++ {
		if cfg.AtePortIPV4 != "" {
			ip, err := IncrementIP(cfg.AtePortIPV4, int(i))
			if err != "" {
				t.Fatalf("Failed to increment IP address with error '%s'", err)
			}
			bgpNbr := &BgpNeighborScale{
				As:         asn,
				Neighborip: ip,
				IsV4:       true,
				Pg:         cfg.PeerV4GrpName,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		if cfg.AtePortIPV6 != "" {
			ip, err := IncrementIP(cfg.AtePortIPV6, int(i))
			if err != "" {
				t.Fatalf("Failed to increment IP address with error '%s'", err)
			}
			bgpNbr := &BgpNeighborScale{
				As:         asn6,
				Neighborip: ip,
				IsV4:       false,
				Pg:         cfg.PeerV6GrpName,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		asn = asn + 1
		asn6 = asn6 + 1
	}
	// Required data to create neighbours is updated in nbrList
	return nbrList
}

func configureBGPScaleOnATE(t *testing.T, top gosnappi.Config, c *EBgpConfigScale) {
	t.Helper()

	devices := top.Devices().Items()
	devMap := make(map[string]gosnappi.Device)
	for _, dev := range devices {
		devMap[dev.Name()] = dev
	}

	var asn uint32 = c.AteASV4
	var asn6 uint32 = c.AteASV6

	for i := uint32(1); i <= c.NumOfPeers; i++ {
		di := c.PortName
		if c.PortName == "port1" {
			di = fmt.Sprintf("%sdst%d.Dev", c.PortName, i)
		}
		device := devMap[di]
		if c.AtePortIPV4 != "" {
			bgp := device.Bgp().SetRouterId(c.AtePortIPV4)
			ipv4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(device.Name() + ".BGP4.peer")
			bgp4Peer.SetPeerAddress(ipv4.Gateway())
			bgp4Peer.SetAsNumber(asn)
			bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
		}
		if c.AtePortIPV6 != "" {
			bgp := device.Bgp().SetRouterId(c.AtePortIPV4)
			ipv6 := device.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(device.Name() + ".BGP6.peer")
			bgp6Peer.SetPeerAddress(ipv6.Gateway())
			bgp6Peer.SetAsNumber(asn6)
			bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			bgp6Peer.Capability().SetIpv6UnicastAddPath(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
		}
		asn = asn + 1
		asn6 = asn6 + 1
	}
}

// ConfigureEBgpPeersScale configures EBGP peers between DUT and ATE ports.
func ConfigureEBgpPeersScale(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config,
	cfg []*EBgpConfigScale,
) (gosnappi.Config, *oc.NetworkInstance_Protocol) {
	var nbrList []*BgpNeighborScale
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	for _, c := range cfg {
		nbr := BuildIPv4v6NbrScale(t, c)
		nbrList = append(nbrList, nbr...)
		pgv4 := bgp.GetOrCreatePeerGroup(c.PeerV4GrpName)
		pgv4.PeerAs = ygot.Uint32(c.AteASV4)
		pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		pgv6 := bgp.GetOrCreatePeerGroup(c.PeerV6GrpName)
		pgv6.PeerAs = ygot.Uint32(c.AteASV6)
		pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	}

	for _, nbr := range nbrList {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		bgpNbr.PeerAs = ygot.Uint32(nbr.As)
		bgpNbr.Enabled = ygot.Bool(true)
		bgpNbr.PeerGroup = ygot.String(nbr.Pg)
		if nbr.IsV4 {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}

	for _, c := range cfg {
		configureBGPScaleOnATE(t, top, c)
	}

	return top, niProto
}

// IncrementIP increments an IPv4 or IPv6 address by a specified number of addresses.
func IncrementIP(ipStr string, num int) (string, string) {
	ip := net.ParseIP(ipStr)
	err := ""
	if ip == nil {
		err = fmt.Sprintf("invalid IP address: %s", ipStr)
		return "", err
	}

	ipInt := big.NewInt(0)
	if ip.To4() != nil {
		ipInt.SetBytes(ip.To4())
	} else {
		ipInt.SetBytes(ip.To16())
	}

	ipInt.Add(ipInt, big.NewInt(int64(num)))

	var newIP net.IP
	if ip.To4() != nil {
		newIP = net.IP(ipInt.Bytes()).To4()
	} else {
		newIP = net.IP(ipInt.Bytes()).To16()
	}
	return newIP.String(), err
}

// VerifyDUTVrfBGPState verify BGP neighbor status with configured DUT VRF configuration.
func VerifyDUTVrfBGPState(t *testing.T, dut *ondatra.DUTDevice, cfg VrfBGPState) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(cfg.NetworkInstanceName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbrIp := range cfg.NeighborIPs {
		nbrPath := statePath.Neighbor(nbrIp)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP neighbor %s did not reach ESTABLISHED", nbrIp)
			continue
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %s", nbrIp, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbrIp, state, want)
		}

		t.Log("Verifying BGP capabilities.")
		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
		}
		for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[cap] = true
		}
		for cap, present := range capabilities {
			if !present {
				t.Errorf("Capability %v not reported for neighbor %s", cap, nbrIp)
			}
		}
	}
}

type RouteInfo struct {
	VRF         string
	IPType      string
	DefaultName string
}

// VerifyRoutes checks if advertised routes are installed in DUT AFT.
func VerifyRoutes(t *testing.T, dut *ondatra.DUTDevice, routesToAdvertise map[string]RouteInfo) {
	t.Helper()
	for route, info := range routesToAdvertise {
		vrfName := info.VRF
		if vrfName == info.DefaultName {
			vrfName = deviations.DefaultNetworkInstance(dut)
		}
		var ok bool
		switch info.IPType {
		case IPv4:
			aft := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(route)
			_, ok = gnmi.Watch(t, dut, aft.State(), routeTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				return val.IsPresent()
			}).Await(t)

		case IPv6:
			normalizedRoute := strings.Replace(route, "::0/", "::/", 1)
			aft := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv6Entry(normalizedRoute)
			_, ok = gnmi.Watch(t, dut, aft.State(), routeTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				return val.IsPresent()
			}).Await(t)
		}

		if !ok {
			t.Errorf("Route %s is NOT installed in AFT for VRF %q", route, info.VRF)
		} else {
			t.Logf("Route %s successfully installed in AFT for VRF %q", route, info.VRF)
		}
	}
}

// ConfigureBMP applies BMP station configuration on DUT.
func ConfigureBMP(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfgParams BMPConfigParams) {
	t.Helper()
	if deviations.BMPOCUnsupported(dut) {

		switch dut.Vendor() {
		case ondatra.ARISTA:
			bmpConfig := new(strings.Builder)
			fmt.Fprintf(bmpConfig, `
				router bgp %d
				bgp monitoring
				! BMP station
				monitoring station BMP_STN
				update-source %s
				statistics
				connection address %s
				connection mode active port %d
				`, cfgParams.DutAS, cfgParams.Source, cfgParams.StationAddr, cfgParams.StationPort)

			helpers.GnmiCLIConfig(t, dut, bmpConfig.String())
		}
	} else {
		// TODO: BMP support is not yet available, so the code below is commented out and will be enabled once BMP is implemented.
		t.Log("BMP support is not yet available, so the code below is commented out and will be enabled once BMP is implemented.")
		// // === BMP Configuration ===
		// bmp := cfgParams.BGPObj.Global.GetOrCreateBmp()
		// bmp.LocalAddress = ygot.String(cfgParams.LocalAddr)
		// bmp.StatisticsTimeout = ygot.Uint16(cfgParams.StatsTimeOut)

		// // --- Create BMP Station ---
		// st := bmp.GetOrCreateStation("BMP_STN")
		// st.Address = ygot.String(cfgParams.StationAddr)
		// st.Port = ygot.Uint16(cfgParams.StationPort)
		// st.ConnectionMode = oc.BgpTypes_BMPStationMode_ACTIVE
		// st.Description = ygot.String("ATE BMP station")
		// st.PolicyType = oc.BgpTypes_BMPPolicyType_POST_POLICY
		// st.ExcludeNoneligible = ygot.Bool(true)
		// // Push configuration
		// gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bmp)
	}
}

// ConfigureBMPAccessList applies access-list to permit BMP port if blocked by default rules.
func ConfigureBMPAccessList(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfgParams BMPConfigParams) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		bmpAclConfig := new(strings.Builder)
		fmt.Fprintf(bmpAclConfig, `
			ip access-list restrict-access
			permit tcp any any eq %[1]d
 			permit tcp any eq %[1]d any`, cfgParams.StationPort)

		helpers.GnmiCLIConfig(t, dut, bmpAclConfig.String())
	}
}
