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
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
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

	// DutAS dut AS
	DutAS = uint32(65656)
	// AteAS1 for ATE port1
	AteAS1 = uint32(65536)
	// AteAS2 for ATE port2
	AteAS2 = uint32(65537)
	// AteAS3 for ATE port3
	AteAS3 = uint32(65538)
	// AteAS4 for ATE port4
	AteAS4 = uint32(65539)

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
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: plenIPv6,
	}
	dutPort2 = &attrs.Attributes{
		Name:    "port2",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: plenIPv6,
	}
	dutPort3 = &attrs.Attributes{
		Name:    "port3",
		IPv4:    "192.0.2.9",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: plenIPv6,
	}
	dutPort4 = &attrs.Attributes{
		Name:    "port4",
		IPv4:    "192.0.2.13",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:13",
		IPv6Len: plenIPv6,
	}

	atePort1 = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: plenIPv6,
	}
	atePort2 = &attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: plenIPv6,
	}
	atePort3 = &attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: plenIPv6,
	}
	atePort4 = &attrs.Attributes{
		Name:    "port4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: plenIPv4,
		IPv6:    "2001:0db8::192:0:2:e",
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

	DUTPorts []*attrs.Attributes
	ATEPorts []*attrs.Attributes
	aftType  oc.E_BgpTypes_AFI_SAFI_TYPE
}

// NewBGPSession creates a new BGPSession using the default global config, and
// configures the interfaces on the dut and the ate based in given topology port count.
// Only supports 2 and 4 port DUT-ATE topology
func NewBGPSession(t *testing.T, pc PortCount) *BGPSession {
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
	return conf
}

// WithEBGP adds eBGP specific config
func (bs *BGPSession) WithEBGP(t *testing.T, aftype oc.E_BgpTypes_AFI_SAFI_TYPE, isSamePG, isSameAS bool) *BGPSession {
	if aftype != oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST && aftype != oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST {
		t.Fatalf("Unsupported AFI type: %v", bs.aftType)
	}
	bs.aftType = aftype

	asNumbers := []uint32{AteAS1, AteAS2, AteAS3, AteAS4}
	if isSameAS {
		asNumbers = []uint32{AteAS1, AteAS1, AteAS1, AteAS1}
	}

	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)
	for i, otgPort := range bs.ATEPorts {
		bgp := devices[i].Bgp().SetRouterId(otgPort.IPv4)

		switch aftype {
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
			ipv4 := devices[i].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(devices[i].Name() + ".BGP4.peer")
			bgp4Peer.SetPeerAddress(ipv4.Gateway())
			bgp4Peer.SetAsNumber(uint32(asNumbers[i]))
			bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
			ipv6 := devices[i].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(devices[i].Name() + ".BGP6.peer")
			bgp6Peer.SetPeerAddress(ipv6.Gateway())
			bgp6Peer.SetAsNumber(uint32(asNumbers[i]))
			bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		}
	}

	dni := bs.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(bs.DUT))
	dni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	niProtocol := dni.GetOrCreateProtocol(PTBGP, bgpName)
	neighborConfig := BuildNeigborConfig(isSamePG, isSameAS, len(bs.DUTPorts))
	niProtocol.Bgp = BuildBGPOCConfig(t, bs.DUT, dutPort1.IPv4, aftype, neighborConfig)

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
			fptest.AssignToNetworkInstance(t, bs.DUT, bs.OndatraDUTPorts[i].Name(), deviations.DefaultNetworkInstance(bs.DUT), 0)
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

	switch bs.aftType {
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
		otgutils.WaitForARP(t.(*testing.T), otg, bs.ATETop, "IPv4")
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
		otgutils.WaitForARP(t.(*testing.T), otg, bs.ATETop, "IPv6")
	}
}

// VerifyDUTBGPEstablished verifies on DUT BGP peer establishment
func (bs *BGPSession) VerifyDUTBGPEstablished(t *testing.T) {
	dni := deviations.DefaultNetworkInstance(bs.DUT)
	nSessionState := gnmi.OC().NetworkInstance(dni).Protocol(PTBGP, bgpName).Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, bs.DUT, nSessionState, 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
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
func (bs *BGPSession) VerifyOTGBGPEstablished(t *testing.T) {
	pSessionState := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, bs.ATE.OTG(), pSessionState, 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
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
	IPv4Neighbor string
	IPv6Neighbor string
	PeerGroup    string
	AS           uint32
}

// BuildNeigborConfig builds neighbor config based on given flags
func BuildNeigborConfig(isSamePG, isSameAS bool, portCount int) []*NeighborConfig {
	nc := []*NeighborConfig{
		{
			IPv4Neighbor: atePort1.IPv4,
			IPv6Neighbor: atePort1.IPv6,
			PeerGroup:    BGPPeerGroup1,
			AS:           AteAS1,
		},
		{
			IPv4Neighbor: atePort2.IPv4,
			IPv6Neighbor: atePort2.IPv6,
			PeerGroup:    BGPPeerGroup2,
			AS:           AteAS2,
		},
	}

	if portCount == int(PortCount4) {
		nc = append(
			nc,
			&NeighborConfig{
				IPv4Neighbor: atePort3.IPv4,
				IPv6Neighbor: atePort3.IPv6,
				PeerGroup:    BGPPeerGroup3,
				AS:           AteAS3,
			},
			&NeighborConfig{
				IPv4Neighbor: atePort4.IPv4,
				IPv6Neighbor: atePort4.IPv6,
				PeerGroup:    BGPPeerGroup4,
				AS:           AteAS4,
			},
		)
	}

	if isSamePG {
		for _, n := range nc {
			n.PeerGroup = BGPPeerGroup1
		}
	}
	if isSameAS {
		for _, n := range nc {
			n.AS = AteAS1
		}
	}

	return nc
}

// BuildBGPOCConfig builds the BGP OC config applying global, neighbors and peer-group config
func BuildBGPOCConfig(t *testing.T, dut *ondatra.DUTDevice, routerID string, aftType oc.E_BgpTypes_AFI_SAFI_TYPE, neighborConfig []*NeighborConfig) *oc.NetworkInstance_Protocol_Bgp {
	afiSafiGlobal := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
		aftType: {
			AfiSafiName: aftType,
			Enabled:     ygot.Bool(true),
		},
	}
	afiSafiNeighbor := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
		aftType: {
			AfiSafiName: aftType,
			Enabled:     ygot.Bool(true),
		},
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
		switch aftType {
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
			neighbor = nc.IPv4Neighbor
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
			neighbor = nc.IPv6Neighbor
		default:
			t.Fatalf("Unsupported AFI type: %v", aftType)
		}

		neighbors[neighbor] = &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			PeerAs:          ygot.Uint32(nc.AS),
			PeerGroup:       ygot.String(nc.PeerGroup),
			NeighborAddress: ygot.String(neighbor),
			AfiSafi:         afiSafiNeighbor,
		}

		peerGroups[nc.PeerGroup] = getPeerGroup(nc.PeerGroup, dut, aftType)
	}

	return &oc.NetworkInstance_Protocol_Bgp{
		Global:    global,
		Neighbor:  neighbors,
		PeerGroup: peerGroups,
	}
}

// getPeerGroup build peer-config
func getPeerGroup(pgn string, dut *ondatra.DUTDevice, aftype oc.E_BgpTypes_AFI_SAFI_TYPE) *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
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
	afisafi := pg.GetOrCreateAfiSafi(aftype)
	afisafi.Enabled = ygot.Bool(true)
	rpl := afisafi.GetOrCreateApplyPolicy()
	rpl.SetExportPolicy([]string{RPLPermitAll})
	rpl.SetImportPolicy([]string{RPLPermitAll})
	return pg
}
