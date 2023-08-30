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

package bgp_tcp_mss_path_mtu_otg_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
// 	ATE   Port-1 192.0.2.2 --------  DUT-1 Port-1 192.0.2.1
//	DUT-1 Port-2 192.0.2.5 --------  DUT-2 Port-1 192.0.2.6

const (
	peerGrpName1       = "BGP-PEER-GROUP1"
	dut1AS             = 65501
	ateAS1             = 65502
	dut2AS             = 65502
	plenIPv4           = 30
	plenIPv6           = 126
	dut1TcpMssDefault  = uint16(536)
	dut1Tcp6MssDefault = uint16(1240)
	dut1IntfMtu        = uint16(5040)
	dut1TcpMss         = uint16(4096)
	mtu512             = uint16(512)
	vlan10             = 10
	vlan20             = 20
	isisInstance       = "DEFAULT"
	authPWd            = "BGPTCPMSS"
	dut1AreaAddress    = "49.0001"
	dut1SysID          = "1920.0000.2001"
	dut2AreaAddress    = "49.0001"
	dut2SysID          = "1920.0000.3001"
	ATESysID           = "640000000001"
)

var (
	dut1Port1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dut1Port2 = attrs.Attributes{
		Desc:    "DUT1 to DUT2",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
	}
	dut2Port1 = attrs.Attributes{
		Name:    "DUT2 to DUT1",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T) {
	dc := gnmi.OC()
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	t.Log("Configure interfaces on dut1.")
	i1 := dut1Port1.NewOCInterface(dut1.Port(t, "port1").Name(), dut1)
	gnmi.Replace(t, dut1, dc.Interface(i1.GetName()).Config(), i1)
	i2 := dut1Port2.NewOCInterface(dut1.Port(t, "port2").Name(), dut1)
	gnmi.Replace(t, dut1, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure interfaces on dut2.")
	i3 := dut2Port1.NewOCInterface(dut2.Port(t, "port1").Name(), dut2)
	gnmi.Replace(t, dut2, dc.Interface(i3.GetName()).Config(), i3)
}

// bgpCreateNbr creates bgp configuration on dut device.
func bgpCreateNbr(t *testing.T, dut *ondatra.DUTDevice, authPwd, routerId string, localAs uint32, nbrs []*bgpNeighbor) *oc.NetworkInstance_Protocol {

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni_proto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := ni_proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(routerId)
	global.As = ygot.Uint32(localAs)

	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS1)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	for _, nbr := range nbrs {
		nv := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv.PeerGroup = ygot.String(nbr.peerGrp)
		nv.PeerAs = ygot.Uint32(nbr.as)
		nv.Enabled = ygot.Bool(true)
		if nbr.isV4 {
			if authPwd != "" {
				nv.AuthPassword = ygot.String(authPWd)
			}
			af4 := nv.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af6 := nv.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return ni_proto
}

// verifyBGPTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbrIP []string) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
		t.Logf("BGP adjacency for %s: %s", nbr, state)
	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName []string, dutAreaAddress, dutSysID string) {
	d := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	for _, intf := range intfName {
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), prot)
}

func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {

	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dut1Port1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dut1Port1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	isisDut1 := iDut1Dev.Isis().SetName("ISIS").SetSystemId(ATESysID)

	isisDut1.Basic().SetIpv4TeRouterId(atePort1.IPv4).SetHostname(isisDut1.Name()).SetLearnedLspFilter(true)
	isisDut1.Interfaces().Add().SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp6 := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp6Peer := iDut1Bgp6.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	t.Logf("Pushing config to OTG and starting protocols...")

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

func configOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {

	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dut1Port1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dut1Port1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	isisDut1 := iDut1Dev.Isis().SetName("ISIS").SetSystemId(ATESysID)

	isisDut1.Basic().SetIpv4TeRouterId(atePort1.IPv4).SetHostname(isisDut1.Name()).SetLearnedLspFilter(true)
	isisDut1.Interfaces().Add().SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(dut2Port1.IPv4).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.IBGP).
		Advanced().SetMd5Key(authPWd)

	t.Logf("Pushing config to OTG and starting protocols...")

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

// TestTcpMssPathMtu is to Validate TCP MSS for BGP v4/v6 sessions.
func TestTcpMssPathMtu(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ate := ondatra.ATE(t, "ate")

	dutList := []*ondatra.DUTDevice{dut1, dut2}

	t.Run("Configure DUT1 and DUT2 interfaces", func(t *testing.T) {
		configureDUT(t)
	})

	t.Run("Configure DEFAULT network instance on both DUT1 and DUT2", func(t *testing.T) {
		for _, dut := range dutList {
			dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
			gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
		}
	})

	dut1ConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut1)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dut2ConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut2)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	t.Run("Configure BGP Neighbors DUT1 - ATE", func(t *testing.T) {
		t.Logf("Start DUT1 BGP Config.")
		gnmi.Delete(t, dut1, dut1ConfPath.Config())
		dut1Nbr1v4 := &bgpNeighbor{as: ateAS1, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
		dut1Nbr1v6 := &bgpNeighbor{as: ateAS1, neighborip: atePort1.IPv6, isV4: false, peerGrp: peerGrpName1}
		dut1Nbrs := []*bgpNeighbor{dut1Nbr1v4, dut1Nbr1v6}
		dut1Conf := bgpCreateNbr(t, dut1, "", dut1Port2.IPv4, dut1AS, dut1Nbrs)
		gnmi.Replace(t, dut1, dut1ConfPath.Config(), dut1Conf)
		fptest.LogQuery(t, "DUT1 BGP Config", dut1ConfPath.Config(), gnmi.GetConfig(t, dut1, dut1ConfPath.Config()))
	})

	otg := ate.OTG()
	var otgConfig gosnappi.Config

	t.Run("configureOTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg)
	})

	var dut1NbrIP = []string{atePort1.IPv4, atePort1.IPv6}
	t.Run("Verify BGP telemetry", func(t *testing.T) {
		verifyBGPTelemetry(t, dut1, dut1NbrIP)
	})

	dut1Port1Path := gnmi.OC().Interface(dut1.Port(t, "port1").Name())
	t.Run("Verify default TCP MSS v4 and v6 session", func(t *testing.T) {
		// TODO:
	})
	t.Run("Change the Interface MTU to the ATE port as 5040.", func(t *testing.T) {
		t.Logf("Configure DUT1 interface MTU to %v", dut1IntfMtu)
		gnmi.Replace(t, dut1, dut1Port1Path.Mtu().Config(), dut1IntfMtu)
		t.Logf("Configure DUT1 BGP TCP-MSS value to %v", dut1TcpMss)
		gnmi.Replace(t, dut1, dut1ConfPath.Bgp().Neighbor(atePort1.IPv4).Transport().TcpMss().Config(), dut1TcpMss)
		gnmi.Replace(t, dut1, dut1ConfPath.Bgp().Neighbor(atePort1.IPv6).Transport().TcpMss().Config(), dut1TcpMss)
	})

	t.Run("Re-establish the BGP sessions by tcp reset.", func(t *testing.T) {
		otg.StopProtocols(t)
		time.Sleep(20 * time.Second)
		otg.PushConfig(t, otgConfig)
		otg.StartProtocols(t)
	})

	t.Run("Verify BGP telemetry after reset.", func(t *testing.T) {
		verifyBGPTelemetry(t, dut1, dut1NbrIP)
	})

	t.Run("Verify BGP TCP MSS value", func(t *testing.T) {
		t.Logf("Verify DUT1 BGP TCP-MSS value is to %v for both BGP v4 and v6 sessions.", dut1TcpMss)
		if gotTcpMss := gnmi.Get(t, dut1, dut1ConfPath.Bgp().Neighbor(atePort1.IPv4).Transport().TcpMss().State()); gotTcpMss != dut1TcpMss {
			t.Errorf("Obtained TCP MSS for BGP v4 on dut1 is not as expected, got is %v, want %v", gotTcpMss, dut1TcpMss)
		}
		if gotTcp6Mss := gnmi.Get(t, dut1, dut1ConfPath.Bgp().Neighbor(atePort1.IPv6).Transport().TcpMss().State()); gotTcp6Mss != dut1TcpMss {
			t.Errorf("Obtained TCP MSS for BGP v6 peer on dut1 is not as expected, got is %v, want %v", gotTcp6Mss, dut1TcpMss)
		}
	})

	t.Run("Remove configured BGP TCP MSS for v4 and v6 neighbors on DUT1", func(t *testing.T) {
		dut1Port1Path := gnmi.OC().Interface(dut1.Port(t, "port1").Name())
		gnmi.Delete(t, dut1, dut1ConfPath.Config())
		gnmi.Delete(t, dut1, dut1Port1Path.Mtu().Config())
	})

	t.Run("Configure ISIS Neighbors on DUT1 and ATE", func(t *testing.T) {
		dut1PortNames := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name()}
		configureISIS(t, dut1, dut1PortNames, dut1AreaAddress, dut1SysID)
	})

	t.Run("Configure static route on DUT2 to ATE to establish multihop iBGP session", func(t *testing.T) {
		ateIPAddr := fmt.Sprintf("%s/%d", atePort1.IPv4, uint32(32))
		configStaticRoute(t, dut2, ateIPAddr, dut1Port2.IPv4)
	})

	t.Run("Establish iBGP session with MD5 enabled from ATE port-1 to DUT-2 - multihop iBGP", func(t *testing.T) {
		t.Logf("Start DUT2 - ATE Port1 iBGP Config.")
		gnmi.Delete(t, dut2, dut2ConfPath.Config())
		dut2Nbr1v4 := &bgpNeighbor{as: ateAS1, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
		dut2Nbrs := []*bgpNeighbor{dut2Nbr1v4}
		dut2Conf := bgpCreateNbr(t, dut2, authPWd, dut2Port1.IPv4, dut2AS, dut2Nbrs)
		gnmi.Replace(t, dut2, dut2ConfPath.Config(), dut2Conf)
		fptest.LogQuery(t, "DUT2 BGP Config", dut2ConfPath.Config(), gnmi.GetConfig(t, dut2, dut2ConfPath.Config()))
	})

	t.Run("Configure iBGP session ATE Port1 - DUT2", func(t *testing.T) {
		otg.StopProtocols(t)
		otgConfig = configOTG(t, otg)
	})

	t.Run("Verify iBGP session between DUT2 - ATE Port1.", func(t *testing.T) {
		var dut2NbrIP = []string{atePort1.IPv4}
		verifyBGPTelemetry(t, dut2, dut2NbrIP)
	})

	t.Run("Change the MTU on DUT1-DUT2 to 512 bytes and Enable mtu discovery", func(t *testing.T) {
		dut2Port1Path := gnmi.OC().Interface(dut2.Port(t, "port1").Name())
		gnmi.Replace(t, dut2, dut2Port1Path.Mtu().Config(), mtu512)
		gnmi.Replace(t, dut2, dut2ConfPath.Bgp().Neighbor(atePort1.IPv4).Transport().MtuDiscovery().Config(), true)
	})

	t.Run("Re-establish the BGP sessions by tcp reset", func(t *testing.T) {
		otg.StopProtocols(t)
		time.Sleep(20 * time.Second)
		otg.PushConfig(t, otgConfig)
		otg.StartProtocols(t)
	})

	t.Run("Verify iBGP session between DUT2 - ATE Port1.", func(t *testing.T) {
		var dut2NbrIP = []string{atePort1.IPv4}
		verifyBGPTelemetry(t, dut2, dut2NbrIP)
	})

	t.Run("Verify TCP MSS adjusted to below 512 bytes", func(t *testing.T) {
		// TODO:
	})
}
