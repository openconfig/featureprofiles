// Copyright 2021 Google Inc.
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

package base_bgp_session_parameters_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// The testbed consists of ate:port1 -> dut:port1.

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutAttrs = attrs.Attributes{
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	ateAttrs = attrs.Attributes{
		//Desc:    "To DUT",
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip1 ",
		IPv4:    "11.11.11.11",
		IPv4Len: 32,
	}
	dutlo0Attrs1 = attrs.Attributes{
		Desc:    "Loopback ip2",
		IPv4:    "12.12.12.12",
		IPv4Len: 32,
	}
)

const (
	dutAS        = 64500
	ateAS        = 64501
	peerGrpName  = "BGP-PEER-GROUP"
	netInstance  = "DEFAULT"
	loopbackIntf = "lo0"
	holdTime0    = 0
	holdTime100  = 100
	holdTime135  = 135
	ConnTime0    = 0
	ConnTime100  = 100
	NegHoldTime0 = 0
)

//Configure network instance
func configureNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	ni1 := d.GetOrCreateNetworkInstance(netInstance)
	ni1.Type = telemetry.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE(*ygot.Int64(1))

	configureDUTLoop(t, dut, dutlo0Attrs)
	loopIPAddr := getLoopIP(t)

	ni1.RouterId = ygot.String(loopIPAddr)
	dutConfPath := dut.Config().NetworkInstance(netInstance)
	dutConfPath.Replace(t, ni1)
}

//configure loopback ip
func configureDUTLoop(t *testing.T, dut *ondatra.DUTDevice, attrs attrs.Attributes) {
	loop1 := attrs.NewInterface(loopbackIntf)
	loop1.Type = telemetry.IETFInterfaces_InterfaceType_softwareLoopback
	dut.Config().Interface(loop1.GetName()).Replace(t, loop1)
}

func getLoopIP(t *testing.T) string {
	dut := ondatra.DUT(t, "dut")
	lo0 := dut.Telemetry().Interface(loopbackIntf).Subinterface(0)
	ipv4Addrs := lo0.Ipv4().AddressAny().Get(t)
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	return ipv4Addrs[0].GetIp()
}

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := dut.Config()
	i1 := dutAttrs.NewInterface(dut.Port(t, "port1").Name())
	dc.Interface(i1.GetName()).Replace(t, i1)
}

// verifyPortsUp asserts that each port on the device is operating
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := dev.Telemetry().Interface(p.Name()).OperStatus().Get(t)
		if want := telemetry.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ate
func bgpCreateNbr(localAs uint32, peerAs uint32, nbrLocalAS uint32, routerID string, AuthPasswd string, holdTime float64, ConnRetTime float64, negHoldTime float64) *telemetry.NetworkInstance_Protocol_Bgp {
	nbr1v4 := &bgpNeighbor{as: peerAs, neighborip: ateAttrs.IPv4, isV4: true}
	nbrs := []*bgpNeighbor{nbr1v4}

	d := &telemetry.Device{}
	ni1 := d.GetOrCreateNetworkInstance(netInstance)
	bgp := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)

	//If explicit router id is provided
	if routerID != "" {
		global.RouterId = ygot.String(routerID)
	}
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerGrpName)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			if nbrLocalAS != 0 {
				nv4.LocalAs = ygot.Uint32(nbrLocalAS)
			}
			if AuthPasswd != "" {
				nv4.AuthPassword = ygot.String(AuthPasswd)
			}
			if holdTime != 0 {
				nv4t := nv4.GetOrCreateTimers()
				nv4t.HoldTime = ygot.Float64(holdTime)
				if negHoldTime != 0 {
					nv4t.NegotiatedHoldTime = ygot.Float64(negHoldTime)
				}
			}

			if ConnRetTime != 0 {
				nv4t := nv4.GetOrCreateTimers()
				nv4t.ConnectRetry = ygot.Float64(ConnRetTime)
			}
			nv4.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		}
	}
	return bgp
}

//Verify BGP capabilities
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Verifying BGP capabilities")
	statePath := dut.Telemetry().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)

	capabilities := map[telemetry.E_BgpTypes_BGP_CAPABILITY]bool{
		telemetry.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
		telemetry.BgpTypes_BGP_CAPABILITY_ASN32:         false,
		telemetry.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
	}
	for _, cap := range nbrPath.SupportedCapabilities().Get(t) {
		capabilities[cap] = true
	}
	for cap, present := range capabilities {
		if !present {
			t.Errorf("Capability not reported: %v", cap)
		}
	}

}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	ifName := dut.Port(t, "port1").Name()
	lastFlapTime := dut.Telemetry().Interface(ifName).LastChange().Get(t)
	t.Logf("Verifying BGP state")
	statePath := dut.Telemetry().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	nbr := statePath.Get(t).GetNeighbor(ateAttrs.IPv4)

	// Get BGP adjacency state
	t.Logf("Waiting for BGP neighbor to establish...")
	_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
		return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
		t.Fatal("No BGP neighbor formed")
	}
	status := nbrPath.SessionState().Get(t)
	t.Logf("BGP adjacency for %s: %s", ateAttrs.IPv4, status)
	if want := telemetry.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
		t.Errorf("BGP peer %s status got %d, want %d", ateAttrs.IPv4, status, want)
	}

	// Check last established timestamp
	lestTime := nbrPath.Get(t).GetLastEstablished()
	t.Logf("BGP last est time :%v, flapTime :%v", lestTime, lastFlapTime)
	if lestTime < lastFlapTime {
		t.Errorf("Bad last-established timestamp: got %v, want >= %v", lestTime, lastFlapTime)
	}

	// Check BGP Transitions
	estTrans := nbr.GetEstablishedTransitions()
	t.Logf("Got established transitions: %d", estTrans)
	if estTrans != 1 {
		t.Errorf("Wrong established-transitions: got %v, want 1", estTrans)
	}

	// Check BGP neighbor address from telemetry
	addrv4 := nbrPath.Get(t).GetNeighborAddress()
	t.Logf("Got ipv4 neighbor address: %s", addrv4)
	if addrv4 != ateAttrs.IPv4 {
		t.Errorf("BGP v4 neighbor address: got %v, want %v", addrv4, ateAttrs.IPv4)
	}
	// Check BGP neighbor address from telemetry
	peerAS := nbrPath.Get(t).GetPeerAs()
	if peerAS != ateAS {
		t.Errorf("BGP peerAs: got %v, want %v", peerAS, ateAS)
	}

	// Check BGP neighbor is enabled
	if !nbrPath.Get(t).GetEnabled() {
		t.Errorf("Expected neighbor %v to be enabled", ateAttrs.IPv4)
	}
}

//Function to configure ATE configs
func configureATE(t *testing.T, localAs uint32, peerIP string, authPwd string, holdTime uint16, keepAlive uint16, connType string) *ondatra.ATETopology {
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()

	iDut1 := topo.AddInterface(ateAttrs.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateAttrs.IPv4CIDR()).WithDefaultGateway(peerIP)
	bgpDut1 := iDut1.BGP()

	if connType == "INTERNAL" {
		bgpDut1.AddPeer().WithPeerAddress(peerIP).WithLocalASN(localAs).WithTypeInternal()
	} else {
		bgpDut1.AddPeer().WithPeerAddress(peerIP).WithLocalASN(localAs).WithTypeExternal()
	}

	if authPwd != "" {
		bgpDut1.AddPeer().WithPeerAddress(peerIP).WithLocalASN(localAs).WithMD5Key(authPwd)
	}
	if holdTime != 0 {
		bgpDut1.AddPeer().WithPeerAddress(peerIP).WithLocalASN(localAs).
			WithHoldTime(holdTime)
	}
	return topo
}

// TestEstablish sets up and verifies basic BGP connection
func TestEstablish(t *testing.T) {
	// DUT configurations.
	t.Logf("Start DUT config load:")
	dut := ondatra.DUT(t, "dut")

	// Configure interface on the DUT
	t.Logf("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure BGP Neighbor on the DUT
	t.Logf("Start DUT Network Instance and BGP Config")
	configureNetworkInstance(t)

	dutConfPath := dut.Config().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
	dutConfPath.Replace(t, nil)
	dutConf := bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime0, ConnTime0, NegHoldTime0)
	dutConfPath.Replace(t, dutConf)

	// ATE Configuration.
	t.Logf("Start ATE Config")
	topo := configureATE(t, ateAS, dutAttrs.IPv4, "", 0, 0, "EXTERNAL")
	topo.Push(t)
	topo.StartProtocols(t)

	// Verify Port Status
	t.Logf("Verifying port status")
	verifyPortsUp(t, dut.Device)

	t.Logf("Check BGP parameters")
	verifyBgpTelemetry(t, dut)

	t.Logf("Check BGP Capabilities")
	verifyBGPCapabilities(t, dut)
}

// TestDisconnect is send cease notification from ate and verify reiceved error code on DUT
func TestDisconnect(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Configure interface on the DUT
	t.Logf("Start DUT interface Config")
	configureDUT(t, dut)

	dutConfPath := dut.Config().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := dut.Telemetry().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)

	// Configure BGP Neighbor on the DUT
	t.Logf("Start DUT Network Instance and BGP Config")
	configureNetworkInstance(t)
	// Clear any existing config
	fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
	dutConfPath.Replace(t, nil)
	dutConf := bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime0, ConnTime0, NegHoldTime0)
	dutConfPath.Replace(t, dutConf)

	t.Logf("configure port and BGP configs on ATE")
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()
	iDut1 := topo.AddInterface(ateAttrs.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateAttrs.IPv4CIDR()).WithDefaultGateway(dutAttrs.IPv4)
	bgpDut1 := iDut1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutAttrs.IPv4).WithLocalASN(ateAS).WithTypeExternal()

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	t.Logf("Verify BGP session state : ESTABLISHED")
	nbrPath.SessionState().Await(t, time.Second*100, telemetry.Bgp_Neighbor_SessionState_ESTABLISHED)

	//TO DO: WAITING FOR THE FIX FROM ATE TO SEND NOTIFICATION TO DUT TO VERIFY ERROR CODE ON DUT

	/*t.Logf("Send Notification from ATE ")
	// ADD CODE HERE TO SEND NOTIFICATION FROM ATE

	t.Logf("Verify BGP session state : ACTIVE")
	nbrPath.SessionState().Await(t, time.Second*100, telemetry.Bgp_Neighbor_SessionState_ACTIVE)

	t.Logf("Verify Error code received on DUT: BgpTypes_BGP_ERROR_CODE_CEASE")
	code := nbrPath.Messages().Received().LastNotificationErrorCode().Get(t)
	if code != telemetry.BgpTypes_BGP_ERROR_CODE_CEASE {
		t.Errorf("On disconnect: expected error code %v, got %v", telemetry.BgpTypes_BGP_ERROR_CODE_CEASE, code)
	}*/

	//clean config on DUT and ATE
	topo.StopProtocols(t)
	dutConfPath.Replace(t, nil)
}

// TestParameters is to verify BGP connection parameters like timers, router-id, md5 authentication.
func TestParameters(t *testing.T) {
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut")

	//Configure network instance with network-instance type and router-id
	configureNetworkInstance(t)

	dutConfPath := dut.Config().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := dut.Telemetry().NetworkInstance(netInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateIP)

	//configure loopback to use it for explicit router id
	configureDUTLoop(t, dut, dutlo0Attrs1)
	loopIPAddr := getLoopIP(t)

	cases := []struct {
		name      string
		dutConf   *telemetry.NetworkInstance_Protocol_Bgp
		ateConf   *ondatra.ATETopology
		wantState *telemetry.NetworkInstance_Protocol_Bgp
		skipMsg   string
	}{
		{
			name:    "basic internal",
			dutConf: bgpCreateNbr(dutAS, dutAS, 0, "", "", holdTime0, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, dutAS, dutIP, "", 0, 0, "INTERNAL"),
		},
		{
			name:    "basic external",
			dutConf: bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime0, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "", 0, 0, "EXTERNAL"),
		},
		{
			name:    "explicit AS",
			dutConf: bgpCreateNbr(dutAS, ateAS, 100, "", "", holdTime0, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "", 0, 0, "EXTERNAL"),
		},
		{
			name:    "explicit router id",
			dutConf: bgpCreateNbr(dutAS, ateAS, 0, loopIPAddr, "", holdTime0, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "", 0, 0, "EXTERNAL"),
		},
		{
			name:    "password",
			dutConf: bgpCreateNbr(dutAS, ateAS, 0, "", "AUTHPASSWORD", holdTime0, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "AUTHPASSWORD", 0, 0, "EXTERNAL"),
		},
		{
			name:    "hold-time, keepalive timer",
			dutConf: bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime100, ConnTime0, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "", 100, 0, "EXTERNAL"),
		},
		{
			name:    "connect-retry",
			dutConf: bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime0, ConnTime100, NegHoldTime0),
			ateConf: configureATE(t, ateAS, dutIP, "", 0, 0, "EXTERNAL"),
		},
		{
			name:      "hold time negotiated",
			dutConf:   bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime100, ConnTime0, NegHoldTime0),
			ateConf:   configureATE(t, ateAS, dutIP, "", 135, 0, "EXTERNAL"),
			wantState: bgpCreateNbr(dutAS, ateAS, 0, "", "", holdTime100, ConnTime0, holdTime100),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.skipMsg) > 0 {
				t.Skip(tc.skipMsg)
			}
			fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
			t.Logf("Clear BGP Configs on DUT")
			dutConfPath.Replace(t, nil)
			t.Logf("Configure BGP Configs on DUT")
			dutConfPath.Replace(t, tc.dutConf)
			t.Logf("Configure BGP on ATE")
			tc.ateConf.Push(t)
			tc.ateConf.StartProtocols(t)
			t.Logf("Verify BGP session state : ESTABLISHED")
			nbrPath.SessionState().Await(t, time.Second*150, telemetry.Bgp_Neighbor_SessionState_ESTABLISHED)
			stateDut := statePath.Get(t)
			wantState := tc.wantState
			if wantState == nil {
				// if you don't specify a wantState, we just validate that the state corresponding to each config value is what it should be.
				wantState = tc.dutConf
			}
			confirm.State(t, wantState, stateDut)
			t.Logf("Clear BGP Configs on ATE")
			tc.ateConf.StopProtocols(t)
		})
	}
}
