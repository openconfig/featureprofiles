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
	"sort"
	"strconv"
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
	PTBGP = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
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
}

// BGPNeighborConfig holds params for creating BGP neighbors + peer groups.
type BGPNeighborConfig struct {
	AteAS            uint32
	PortName         string
	NeighborIPv4     string
	NeighborIPv6     string
	IsLag            bool
	MultiPathEnabled bool
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
func VerifyDUTBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	nSessionState := gnmi.OC().NetworkInstance(dni).Protocol(PTBGP, bgpName).Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, dut, nSessionState, 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
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
func VerifyOTGBGPEstablished(t *testing.T, ate *ondatra.ATEDevice) {
	pSessionState := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, ate.OTG(), pSessionState, 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
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

// UpdateNeighborMaxPrefix updates neighbor max prefixes for BGPMissingOCMaxPrefixesConfiguration deviation.
func UpdateNeighborMaxPrefix(t *testing.T, dut *ondatra.DUTDevice, neighbors []*BgpNeighbor) {
	for _, nbr := range neighbors {
		DeviationAristaBGPNeighborMaxPrefixes(t, dut, nbr.Neighborip, 0)
	}
}

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

// CreateBGPNeighbors creates BGP neighbors for the given router ID, peer group names, and
// neighbors. The global AS and router ID are set to the AS and router ID of the first neighbor,
// assuming that all neighbors provided have the same local AS and the same peer AS.
func CreateBGPNeighbors(t *testing.T, routerID, peerGrpNameV4, peerGrpNameV6 string, nbrs []*BgpNeighbor, dut *ondatra.DUTDevice) (*oc.NetworkInstance_Protocol, error) {
	if len(nbrs) == 0 {
		t.Logf("No BGP neighbors found for router ID: %s, peer group names: %s, peer group names: %s", routerID, peerGrpNameV4, peerGrpNameV6)
		return nil, nil
	}
	if !sameAS(nbrs) {
		return nil, fmt.Errorf("BGP neighbors have different AS numbers: %v", nbrs)
	}
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProtocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(nbrs[0].LocalAS)
	global.SetRouterId(routerID)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	peerGroupV4 := bgp.GetOrCreatePeerGroup(peerGrpNameV4)
	peerGroupV4.SetPeerAs(nbrs[0].PeerAS)
	peerGroupV6 := bgp.GetOrCreatePeerGroup(peerGrpNameV6)
	peerGroupV6.SetPeerAs(nbrs[0].PeerAS)

	afiSAFI := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSAFI.SetEnabled(true)
	asisafi6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	asisafi6.SetEnabled(true)

	peerGroupV4AfiSafi := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	peerGroupV4AfiSafi.SetEnabled(true)
	peerGroupV6AfiSafi := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	peerGroupV6AfiSafi.SetEnabled(true)

	if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
		peerGroupV4.GetOrCreateUseMultiplePaths().SetEnabled(true)
		peerGroupV6.GetOrCreateUseMultiplePaths().SetEnabled(true)
	} else {
		afiSAFI.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
		asisafi6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(2)
		peerGroupV4AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
		peerGroupV6AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)
	}
	for _, nbr := range nbrs {
		neighbor := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		neighbor.SetPeerAs(nbr.PeerAS)
		neighbor.SetEnabled(true)
		switch {
		case nbr.IsV4:
			neighbor.SetPeerGroup(peerGrpNameV4)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
			neighbourAFV4 := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			neighbourAFV4.SetEnabled(true)
			applyPolicy := neighbourAFV4.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{ALLOW}
			applyPolicy.ExportPolicy = []string{ALLOW}
		case !nbr.IsV4:
			neighbor.SetPeerGroup(peerGrpNameV6)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
			neighbourAFV6 := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			neighbourAFV6.SetEnabled(true)
			applyPolicy := neighbourAFV6.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{ALLOW}
			applyPolicy.ExportPolicy = []string{ALLOW}
		}
	}
	return niProtocol, nil
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

	// Handle multipath deviation
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
	rpl4.ImportPolicy = []string{ALLOW}
	rpl4.ExportPolicy = []string{ALLOW}

	// === Peer Group for IPv6 ===
	pgv6Name := cfg.PortName + "BGP-PEER-GROUP-V6"
	pgv6 := bgp.GetOrCreatePeerGroup(pgv6Name)
	pgv6.PeerAs = ygot.Uint32(cfg.AteAS)
	pgv6.PeerGroupName = ygot.String(pgv6Name)
	pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgafv6.Enabled = ygot.Bool(true)
	rpl6 := pgafv6.GetOrCreateApplyPolicy()
	rpl6.ImportPolicy = []string{ALLOW}
	rpl6.ExportPolicy = []string{ALLOW}

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
