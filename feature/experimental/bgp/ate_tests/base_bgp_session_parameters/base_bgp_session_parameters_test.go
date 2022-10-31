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

package base_bgp_session_parameters_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// The testbed consists of ate:port1 -> dut:port1.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// List of variables.
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
)

// Constants.
const (
	dutAS            = 65540
	ateAS            = 65550
	dutAS2           = 65536
	ateAS2           = 65536
	peerGrpName      = "BGP-PEER-GROUP"
	authPassword     = "AUTHPASSWORD"
	dutHoldTime      = 90
	connRetryTime    = 100
	ateHoldTime      = 135
	dutKeepaliveTime = 30
)

type connType string

const (
	connInternal connType = "INTERNAL"
	connExternal connType = "EXTERNAL"
)

// configureDUT is used to configure interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := dut.Config()
	i1 := dutAttrs.NewInterface(dut.Port(t, "port1").Name())
	dc.Interface(i1.GetName()).Replace(t, i1)
}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := dev.Telemetry().Interface(p.Name()).OperStatus().Get(t)
		if want := telemetry.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// Struct is to pass bgp session parameters.
type bgpTestParams struct {
	localAS, peerAS, nbrLocalAS uint32
	peerIP                      string
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ate and returns bgp object.
func bgpCreateNbr(bgpParams *bgpTestParams) *telemetry.NetworkInstance_Protocol_Bgp {
	d := &telemetry.Device{}
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	bgp := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(bgpParams.localAS)

	global.RouterId = ygot.String(dutAttrs.IPv4)
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(bgpParams.peerAS)
	pg.PeerGroupName = ygot.String(peerGrpName)

	nv4 := bgp.GetOrCreateNeighbor(ateAttrs.IPv4)
	nv4.PeerGroup = ygot.String(peerGrpName)
	nv4.PeerAs = ygot.Uint32(bgpParams.peerAS)
	nv4.Enabled = ygot.Bool(true)

	if bgpParams.nbrLocalAS != 0 {
		nv4.LocalAs = ygot.Uint32(bgpParams.nbrLocalAS)
	}

	nv4.AuthPassword = ygot.String(authPassword)

	nv4t := nv4.GetOrCreateTimers()
	nv4t.HoldTime = ygot.Uint16(dutHoldTime)
	nv4t.KeepaliveInterval = ygot.Uint16(dutKeepaliveTime)
	nv4t.ConnectRetry = ygot.Uint16(connRetryTime)

	nv4.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	return bgp
}

// Verify BGP capabilities like route refresh as32 and mpbgp.
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP capabilities")
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
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

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	ifName := dut.Port(t, "port1").Name()
	lastFlapTime := dut.Telemetry().Interface(ifName).LastChange().Get(t)
	t.Log("Verifying BGP state")
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	nbr := statePath.Get(t).GetNeighbor(ateAttrs.IPv4)

	// Get BGP adjacency state
	t.Log("Waiting for BGP neighbor to establish...")
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

// Function to configure ATE configs based on args and returns ate topology handle.
func configureATE(t *testing.T, ateParams *bgpTestParams, connectionType connType) *ondatra.ATETopology {
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()

	iDut1 := topo.AddInterface(ateAttrs.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateAttrs.IPv4CIDR()).WithDefaultGateway(ateParams.peerIP)
	bgpDut1 := iDut1.BGP()

	if connectionType == connInternal {
		bgpDut1.AddPeer().WithPeerAddress(ateParams.peerIP).WithLocalASN(ateParams.localAS).WithTypeInternal().
			WithMD5Key(authPassword).WithHoldTime(ateHoldTime)
	} else {
		bgpDut1.AddPeer().WithPeerAddress(ateParams.peerIP).WithLocalASN(ateParams.localAS).WithTypeExternal().
			WithMD5Key(authPassword).WithHoldTime(ateHoldTime)
	}
	return topo

}

// TestEstablishAndDisconnect Establishes BGP session between DUT and ATE and Verifies
// abnormal termination of session using notification message:
func TestEstablishAndDisconnect(t *testing.T) {
	// DUT configurations.
	t.Log("Start DUT config load:")
	dut := ondatra.DUT(t, "dut")

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure Network instance type on DUT
	t.Log("Configure Network Instance")
	dutConfNIPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Log("Configure BGP")
	dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)

	fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
	dutConfPath.Replace(t, nil)
	dutConf := bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS})
	dutConfPath.Replace(t, dutConf)
	fptest.LogYgot(t, "DUT BGP Config", dutConfPath, dutConfPath.Get(t))

	// ATE Configuration.
	t.Log("configure port and BGP configs on ATE")
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()
	iDut1 := topo.AddInterface(ateAttrs.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateAttrs.IPv4CIDR()).WithDefaultGateway(dutAttrs.IPv4)
	bgpDut1 := iDut1.BGP()
	bgpPeer := bgpDut1.AddPeer().WithPeerAddress(dutAttrs.IPv4).WithLocalASN(ateAS).WithTypeExternal().
		WithMD5Key(authPassword).WithHoldTime(ateHoldTime)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	// Verify Port Status
	t.Log("Verifying port status")
	verifyPortsUp(t, dut.Device)

	// Verify BGP status
	t.Log("Check BGP parameters")
	verifyBgpTelemetry(t, dut)

	// Verify BGP capabilities
	t.Log("Check BGP Capabilities")
	verifyBGPCapabilities(t, dut)

	// Send Cease Notification from ATE to DUT
	t.Log("Send Cease Notification from ATE to DUT")
	ate.Actions().NewBGPPeerNotification().WithCode(6).WithSubCode(6).WithPeers(bgpPeer).Send(t)

	// Verify BGP session state : ACTIVE
	t.Log("Verify BGP session state : ACTIVE")
	nbrPath.SessionState().Await(t, time.Second*60, telemetry.Bgp_Neighbor_SessionState_ACTIVE)

	// Verify if Cease notification is received on DUT.
	t.Log("Verify Error code received on DUT: BgpTypes_BGP_ERROR_CODE_CEASE")
	code := nbrPath.Messages().Received().LastNotificationErrorCode().Get(t)
	if code != telemetry.BgpTypes_BGP_ERROR_CODE_CEASE {
		t.Errorf("On disconnect: expected error code %v, got %v", telemetry.BgpTypes_BGP_ERROR_CODE_CEASE, code)
	}

	// Clear config on DUT and ATE
	topo.StopProtocols(t)
	dutConfPath.Replace(t, nil)
}

// TestParameters is to verify normal session establishment and termination
// in both eBGP and iBGP scenarios using session parameters like explicit
// router id , timers and MD5 authentication.
func TestParameters(t *testing.T) {
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut")

	// Configure Network instance type on DUT
	t.Log("Configure Network Instance")
	dutConfNIPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateIP)

	cases := []struct {
		name      string
		dutConf   *telemetry.NetworkInstance_Protocol_Bgp
		ateConf   *ondatra.ATETopology
		wantState *telemetry.NetworkInstance_Protocol_Bgp
	}{
		{
			name:    "Test the eBGP session establishment: Global AS",
			dutConf: bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS}),
			ateConf: configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP}, connExternal),
		},
		{
			name:    "Test the eBGP session establishment: Neighbor AS",
			dutConf: bgpCreateNbr(&bgpTestParams{localAS: dutAS2, peerAS: ateAS, nbrLocalAS: dutAS}),
			ateConf: configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP}, connExternal),
		},
		{
			name:    "Test the iBGP session establishment: Gloabl AS",
			dutConf: bgpCreateNbr(&bgpTestParams{localAS: dutAS2, peerAS: ateAS2}),
			ateConf: configureATE(t, &bgpTestParams{localAS: ateAS2, peerIP: dutIP}, connInternal),
		},
		{
			name:    "Test the iBGP session establishment: Neighbor AS",
			dutConf: bgpCreateNbr(&bgpTestParams{localAS: dutAS2, peerAS: ateAS2, nbrLocalAS: dutAS2}),
			ateConf: configureATE(t, &bgpTestParams{localAS: ateAS2, peerIP: dutIP}, connInternal),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fptest.LogYgot(t, "DUT BGP Config before", dutConfPath, dutConfPath.Get(t))
			t.Log("Clear BGP Configs on DUT")
			dutConfPath.Replace(t, nil)
			t.Log("Configure BGP Configs on DUT")
			dutConfPath.Replace(t, tc.dutConf)
			fptest.LogYgot(t, "DUT BGP Config ", dutConfPath, dutConfPath.Get(t))
			t.Log("Configure BGP on ATE")
			tc.ateConf.Push(t)
			tc.ateConf.StartProtocols(t)
			t.Log("Verify BGP session state : ESTABLISHED")
			nbrPath.SessionState().Await(t, time.Second*100, telemetry.Bgp_Neighbor_SessionState_ESTABLISHED)
			stateDut := statePath.Get(t)
			wantState := tc.dutConf
			confirm.State(t, wantState, stateDut)
			t.Log("Clear BGP Configs on ATE")
			tc.ateConf.StopProtocols(t)
		})
	}
}
