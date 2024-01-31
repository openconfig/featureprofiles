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

package bgp_afi_safi_defaults_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration          = 1 * time.Minute
	ipv4SrcTraffic           = "192.0.2.2"
	advertisedRoutesv4CIDR   = "203.0.113.1/32"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv4Prefix = 32
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.254"
	peerGrpName1             = "BGP-PEER-GROUP1"
	peerGrpName2             = "BGP-PEER-GROUP2"
	tolerancePct             = 2
	tolerance                = 50
	routeCount               = 254
	dutAS                    = 65501
	ateAS                    = 65502
	plenIPv4                 = 30
	plenIPv6                 = 126
	nbrLevel                 = "NEIGHBOR"
	peerGrpLevel             = "PEER-GROUP"
	globalLevel              = "GLOBAL"
	afiSafiSetToFalse        = "AFISAFI-SET-TO-FALSE"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	nbr1 = &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2 = &bgpNeighbor{as: ateAS, neighborip: atePort1.IPv6, isV4: false, peerGrp: peerGrpName2}
	nbr3 = &bgpNeighbor{as: dutAS, neighborip: atePort2.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr4 = &bgpNeighbor{as: dutAS, neighborip: atePort2.IPv6, isV4: false, peerGrp: peerGrpName2}

	otgPort1V4Peer = "atePort1.BGP4.peer"
	otgPort1V6Peer = "atePort1.BGP6.peer"
	otgPort2V4Peer = "atePort2.BGP4.peer"
	otgPort2V6Peer = "atePort2.BGP6.peer"
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst.
func bgpCreateNbr(t *testing.T, localAs, peerAs uint32, dut *ondatra.DUTDevice, afiSafiLevel string, nbrs []*bgpNeighbor, isV4Only bool) *oc.NetworkInstance_Protocol {
	t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.As = ygot.Uint32(localAs)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1) // V4 peer group
	pg1.PeerAs = ygot.Uint32(ateAS)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2) // V6 peer group
	pg2.PeerAs = ygot.Uint32(dutAS)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.peerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)

		switch afiSafiLevel {
		case globalLevel:
			global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
			global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
			if !isV4Only {
				extNh := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast()
				extNh.ExtendedNextHopEncoding = ygot.Bool(true)
			}
			if deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut) {
				global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast = nil
			}
		case nbrLevel:
			if nbr.isV4 == true {
				af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				af4.Enabled = ygot.Bool(true)
			} else {
				af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
				af6.Enabled = ygot.Bool(true)
			}
		case peerGrpLevel:
			pg1af4 := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pg1af4.Enabled = ygot.Bool(true)
			pg1af6 := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			pg1af6.Enabled = ygot.Bool(true)

			pg2af4 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pg2af4.Enabled = ygot.Bool(true)
			ext2Nh := pg2af4.GetOrCreateIpv4Unicast()
			ext2Nh.ExtendedNextHopEncoding = ygot.Bool(true)
			pg2af6 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			pg2af6.Enabled = ygot.Bool(true)
		case afiSafiSetToFalse:
			t.Log("AFI-SAFI is set to false")
			if nbr.isV4 {
				af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
				af4.Enabled = ygot.Bool(false)
			} else {
				af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
				af6.Enabled = ygot.Bool(false)
			}
		}
	}
	return niProto
}

func verifyOtgBgpTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, otgPeerList []string, state string) {
	t.Helper()
	for _, configPeer := range otgPeerList {
		nbrPath := gnmi.OTG().BgpPeer(configPeer)
		_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState.String() == state
		}).Await(t)
		if !ok {
			t.Errorf("No BGP neighbor formed for peer %s", configPeer)
		}
	}
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbrsList []*bgpNeighbor) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrsList {
		nbrPath := bgpPath.Neighbor(nbr.neighborip)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr.neighborip, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr.neighborip, state, want)
		}
	}
}

// verifyBgpSession checks BGP session state.
func verifyBgpSession(t *testing.T, dut *ondatra.DUTDevice, nbrsList []*bgpNeighbor) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrsList {
		nbrPath := bgpPath.Neighbor(nbr.neighborip)
		state := gnmi.Get(t, dut, nbrPath.SessionState().State())
		if state == oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			t.Errorf("BGP peer %s status got %d, want other than ESTABLISHED", nbr.neighborip, state)
		}
	}
}

// configureOTG configures the interfaces and BGP protocols on an ATE, including
// advertising some(faked) networks over BGP.
func configureOTG(t *testing.T, otg *otg.OTG, otgPeerList []string) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())

	// BGP seesion
	for _, peer := range otgPeerList {
		switch peer {
		case otgPort1V4Peer:
			iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(otgPort1V4Peer)
			iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case otgPort1V6Peer:
			iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(otgPort1V6Peer)
			iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			iDut1Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case otgPort2V4Peer:
			iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(otgPort2V4Peer)
			iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case otgPort2V6Peer:
			iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Ipv6.Name()).Peers().Add().SetName(otgPort2V6Peer)
			iDut2Bgp6Peer.SetPeerAddress(iDut2Ipv6.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
			iDut2Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		}
	}

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return config
}

// verifyBGPCapabilities is used to Verify BGP capabilities like route refresh as32 and mpbgp.
func verifyBgpCapabilities(t *testing.T, dut *ondatra.DUTDevice, afiSafiLevel string, nbrs []*bgpNeighbor) {
	t.Helper()
	t.Log("Verifying BGP AFI-SAFI capabilities.")

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	var nbrPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafiPathAny

	for _, nbr := range nbrs {
		nbrPath = statePath.Neighbor(nbr.neighborip).AfiSafiAny()

		capabilities := map[oc.E_BgpTypes_AFI_SAFI_TYPE]bool{
			oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: false,
			oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: false,
		}

		for _, cap := range gnmi.GetAll(t, dut, nbrPath.State()) {
			capabilities[cap.GetAfiSafiName()] = cap.GetActive()
		}

		switch afiSafiLevel {
		case nbrLevel:
			if nbr.isV4 && capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST] {
				t.Errorf("AFI_SAFI_TYPE_IPV6_UNICAST should not be enabled for v4 Peer: %v, %v", capabilities, nbr.neighborip)
			}
			if !nbr.isV4 && capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST] {
				t.Errorf("AFI_SAFI_TYPE_IPV4_UNICAST should not be for v6 Peer: %v, %v", capabilities, nbr.neighborip)
			}
			t.Logf("Capabilities for peer %v are %v", nbr.neighborip, capabilities)
		case peerGrpLevel:
			if capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST] == true &&
				capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST] == true {
				t.Logf("Both V4 and V6 AFI-SAFI are inherited from peer-group level for peer: %v, %v", nbr.neighborip, capabilities)
			} else {
				t.Errorf("Both V4 and V6 AFI-SAFI are not inherited from peer-group level for peer: %v, %v", nbr.neighborip, capabilities)
			}
		case globalLevel:
			if capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST] == true &&
				capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST] == true {
				t.Logf("Both V4 and V6 AFI-SAFI are inherited from global level for peer: %v, %v", nbr.neighborip, capabilities)
			} else {
				t.Errorf("Both V4 and V6 AFI-SAFI are not inherited from global level for peer: %v, %v", nbr.neighborip, capabilities)
			}
		case afiSafiSetToFalse:
			if nbr.isV4 && capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST] == true {
				t.Errorf("AFI-SAFI are Active after disabling: %v, %v", capabilities, nbr.neighborip)
			}
			if !nbr.isV4 && capabilities[oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST] == true {
				t.Errorf("AFI-SAFI are Active after disabling: %v, %v", capabilities, nbr.neighborip)
			}
		}
	}
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
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

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

// TestAfiSafiOcDefaults validates AFI-SAFI configuration enabled at neighbor,
// peer group and global levels.
func TestAfiSafiOcDefaults(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	cases := []struct {
		desc         string
		afiSafiLevel string
		nbrs         []*bgpNeighbor
		isV4Only     bool
		otgPeerList  []string
	}{{
		desc:         "Validate AFI-SAFI OC defaults at neighbor level for BGPv4 peers",
		afiSafiLevel: nbrLevel,
		nbrs:         []*bgpNeighbor{nbr1, nbr3},
		isV4Only:     true,
		otgPeerList:  []string{otgPort1V4Peer, otgPort2V4Peer},
	}, {
		desc:         "Validate AFI-SAFI OC defaults at peer group level for BGPv4 peers",
		afiSafiLevel: peerGrpLevel,
		nbrs:         []*bgpNeighbor{nbr1, nbr3},
		isV4Only:     false,
		otgPeerList:  []string{otgPort1V4Peer, otgPort2V4Peer},
	}, {
		desc:         "Validate AFI-SAFI OC defaults at global level for V4 peers",
		afiSafiLevel: globalLevel,
		nbrs:         []*bgpNeighbor{nbr1, nbr3},
		isV4Only:     true,
		otgPeerList:  []string{otgPort1V4Peer, otgPort2V4Peer},
	}, {
		desc:         "Validate AFI-SAFI OC defaults at neighbor level for BGPv6 peers",
		afiSafiLevel: nbrLevel,
		nbrs:         []*bgpNeighbor{nbr2, nbr4},
		isV4Only:     false,
		otgPeerList:  []string{otgPort1V6Peer, otgPort2V6Peer},
	}, {
		desc:         "Validate AFI-SAFI OC defaults at peer group level for BGPv6 peers",
		afiSafiLevel: peerGrpLevel,
		nbrs:         []*bgpNeighbor{nbr2, nbr4},
		isV4Only:     false,
		otgPeerList:  []string{otgPort1V6Peer, otgPort2V6Peer},
	}, {
		desc:         "Validate AFI-SAFI OC defaults at global level for BGPv6 peers",
		afiSafiLevel: globalLevel,
		nbrs:         []*bgpNeighbor{nbr2, nbr4},
		isV4Only:     false,
		otgPeerList:  []string{otgPort1V6Peer, otgPort2V6Peer},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			t.Run("Configure BGP Neighbors", func(t *testing.T) {
				bgpClearConfig(t, dut)
				dutConf := bgpCreateNbr(t, dutAS, ateAS, dut, tc.afiSafiLevel, tc.nbrs, tc.isV4Only)
				gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
				fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
			})

			otg := ate.OTG()
			var otgConfig gosnappi.Config
			t.Run("Configure OTG", func(t *testing.T) {
				otgConfig = configureOTG(t, otg, tc.otgPeerList)
			})

			t.Run("Verify port status on DUT", func(t *testing.T) {
				verifyPortsUp(t, dut.Device)
			})

			t.Run("Verify BGP telemetry", func(t *testing.T) {
				verifyBgpTelemetry(t, dut, tc.nbrs)
				verifyOtgBgpTelemetry(t, otg, otgConfig, tc.otgPeerList, "ESTABLISHED")
				verifyBgpCapabilities(t, dut, tc.afiSafiLevel, tc.nbrs)
			})
		})
	}
}

// TestAfiSafiSetToFalse validates AFI-SAFI configuration is set to false.
func TestAfiSafiSetToFalse(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	if deviations.SkipBgpSessionCheckWithoutAfisafi(dut) {
		t.Skip("Skip test BGP when AFI-SAFI is disabled...")
	}

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	cases := []struct {
		desc         string
		afiSafiLevel string
		nbrs         []*bgpNeighbor
		isV4Only     bool
		otgPeerList  []string
	}{{
		desc:         "Validate AFI-SAFI Not enabled at any level for BGPv4 peers",
		afiSafiLevel: afiSafiSetToFalse,
		nbrs:         []*bgpNeighbor{nbr1, nbr3},
		isV4Only:     true,
		otgPeerList:  []string{otgPort1V4Peer, otgPort2V4Peer},
	}, {
		desc:         "Validate AFI-SAFI Not enabled at any level for BGPv6 peers",
		afiSafiLevel: afiSafiSetToFalse,
		nbrs:         []*bgpNeighbor{nbr2, nbr4},
		isV4Only:     false,
		otgPeerList:  []string{otgPort1V6Peer, otgPort2V6Peer},
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			t.Run("Configure BGP Neighbors", func(t *testing.T) {
				bgpClearConfig(t, dut)
				dutConf := bgpCreateNbr(t, dutAS, ateAS, dut, tc.afiSafiLevel, tc.nbrs, tc.isV4Only)
				gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
				fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
			})

			otg := ate.OTG()
			t.Run("Configure OTG", func(t *testing.T) {
				configureOTG(t, otg, tc.otgPeerList)
			})

			t.Run("Verify BGP telemetry", func(t *testing.T) {
				verifyBgpSession(t, dut, tc.nbrs)
				verifyBgpCapabilities(t, dut, tc.afiSafiLevel, tc.nbrs)
			})
		})
	}
}
