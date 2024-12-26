// Copyright 2024 Google LLC
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

package bgp_session_mode_configuration_test

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
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
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
		Desc:    "To DUT",
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
)

// Constants.
const (
	dutAS          = 65540
	ateAS          = 65550
	peerGrpName    = "eBGP-PEER-GROUP"
	peerLvlPassive = "PeerGrpLevelPassive"
	peerLvlActive  = "PeerGrpLevelActive"
	nbrLvlPassive  = "nbrLevelPassive"
	nbrLvlActive   = "nbrLevelActive"
)

// configureDUT is used to configure interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutAttrs.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
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

// Struct is to pass bgp session parameters.
type bgpTestParams struct {
	localAS, peerAS, nbrLocalAS uint32
	peerIP                      string
	transportMode               string
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ate and returns bgp object.
func bgpCreateNbr(bgpParams *bgpTestParams, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(bgpParams.localAS)
	global.RouterId = ygot.String(dutAttrs.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(dutAS)
	pg.PeerGroupName = ygot.String(peerGrpName)
	pgT := pg.GetOrCreateTransport()
	pgT.LocalAddress = ygot.String(dutAttrs.IPv4)
	pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	nv4 := bgp.GetOrCreateNeighbor(ateAttrs.IPv4)
	nv4.PeerGroup = ygot.String(peerGrpName)
	nv4.PeerAs = ygot.Uint32(ateAS)
	nv4.Enabled = ygot.Bool(true)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nv4T := nv4.GetOrCreateTransport()
	nv4T.LocalAddress = ygot.String(dutAttrs.IPv4)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	switch bgpParams.transportMode {
	case nbrLvlPassive:
		nv4.GetOrCreateTransport().SetPassiveMode(true)
	case nbrLvlActive:
		nv4.GetOrCreateTransport().SetPassiveMode(false)
	case peerLvlPassive:
		pg.GetOrCreateTransport().SetPassiveMode(true)
	case peerLvlActive:
		pg.GetOrCreateTransport().SetPassiveMode(false)
	}

	return niProto
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

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantState oc.E_Bgp_Neighbor_SessionState, transMode string, transModeOnATE string) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	if deviations.BgpSessionStateIdleInPassiveMode(dut) {
		if transModeOnATE == nbrLvlPassive || transModeOnATE == peerLvlPassive {
			t.Logf("BGP session state idle is supported in passive mode, transMode: %s, transModeOnATE: %s", transMode, transModeOnATE)
			wantState = oc.Bgp_Neighbor_SessionState_IDLE
		}
	}
	// Get BGP adjacency state
	t.Log("Checking BGP neighbor to state...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == wantState
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Errorf("BGP Session state is not as expected.")
	}
	status := gnmi.Get(t, dut, nbrPath.SessionState().State())
	t.Logf("BGP adjacency for %s: %s", ateAttrs.IPv4, status)
	t.Logf("wantState: %s, status: %s", wantState, status)
	if status != wantState {
		t.Errorf("BGP peer %s status got %d, want %d", ateAttrs.IPv4, status, wantState)
	}

	nbrTransMode := gnmi.Get(t, dut, nbrPath.Transport().State())
	pgTransMode := gnmi.Get(t, dut, statePath.PeerGroup(peerGrpName).Transport().State())
	t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())

	// Check transport mode telemetry.
	switch transMode {
	case nbrLvlPassive:
		if nbrTransMode.GetPassiveMode() != true {
			t.Errorf("Neighbor level passive mode is not set to true on DUT. want true, got %v", nbrTransMode.GetPassiveMode())
		}
		t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	case nbrLvlActive:
		if nbrTransMode.GetPassiveMode() != false {
			t.Errorf("Neighbor level passive mode is not set to false on DUT. want false, got %v", nbrTransMode.GetPassiveMode())
		}
		t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	case peerLvlPassive:
		if pgTransMode.GetPassiveMode() != true {
			t.Errorf("Peer group level passive mode is not set to true on DUT. want true, got %v", pgTransMode.GetPassiveMode())
		}
		t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())
	case peerLvlActive:
		if pgTransMode.GetPassiveMode() != false {
			t.Errorf("Peer group level passive mode is not set to false on DUT. want false, got %v", pgTransMode.GetPassiveMode())
		}
		t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())
	}
}

// Function to configure ATE configs based on args and returns ate topology handle.
func configureATE(t *testing.T, ateParams *bgpTestParams) gosnappi.Config {
	t.Helper()
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := gosnappi.NewConfig()

	topo.Ports().Add().SetName(port1.ID())
	dev := topo.Devices().Add().SetName(ateAttrs.Name)
	eth := dev.Ethernets().Add().SetName(ateAttrs.Name + ".Eth")
	eth.Connection().SetPortName(port1.ID())
	eth.SetMac(ateAttrs.MAC)

	ip := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
	ip.SetAddress(ateAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(ateAttrs.IPv4Len))

	bgp := dev.Bgp().SetRouterId(ateAttrs.IPv4)
	peerBGP := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip.Name()).Peers().Add()
	peerBGP.SetName(ateAttrs.Name + ".BGP4.peer")
	peerBGP.SetPeerAddress(ip.Gateway()).SetAsNumber(uint32(ateParams.localAS))
	peerBGP.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	switch ateParams.transportMode {
	case nbrLvlPassive:
		peerBGP.Advanced().SetPassiveMode(true)
	case peerLvlPassive:
		peerBGP.Advanced().SetPassiveMode(true)
	case peerLvlActive:
		peerBGP.Advanced().SetPassiveMode(false)
	case nbrLvlActive:
		peerBGP.Advanced().SetPassiveMode(false)
	}

	return topo
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config) {
	// nbrPath := gnmi.OTG().BgpPeer("ateSrc.BGP4.peer")
	t.Log("OTG telemetry does not support checking transport mode.")
}

// TestBgpSessionModeConfiguration is to verify when transport mode is set
// active/passive at both neighbor level and peer group level.
func TestBgpSessionModeConfiguration(t *testing.T) {
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure Network instance type on DUT
	t.Log("Configure Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Verify Port Status
	t.Log("Verifying port status")
	verifyPortsUp(t, dut.Device)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	cases := []struct {
		name             string
		dutConf          *oc.NetworkInstance_Protocol
		ateConf          gosnappi.Config
		wantBGPState     oc.E_Bgp_Neighbor_SessionState
		dutTransportMode string
		otgTransportMode string
	}{
		{
			name:             "Test transport mode passive at neighbor level on both DUT and ATE ",
			dutConf:          bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS, transportMode: nbrLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: nbrLvlPassive}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ACTIVE,
			dutTransportMode: nbrLvlPassive,
			otgTransportMode: nbrLvlPassive,
		},
		{
			name:             "Test transport mode active on ATE and passive on DUT at neighbor level",
			dutConf:          bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS, nbrLocalAS: dutAS, transportMode: nbrLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: nbrLvlActive}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ESTABLISHED,
			dutTransportMode: nbrLvlPassive,
			otgTransportMode: nbrLvlActive,
		},
		{
			name:             "Test transport passive mode at Peer group level on both DUT and ATE.",
			dutConf:          bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS, transportMode: peerLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: peerLvlPassive}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ACTIVE,
			dutTransportMode: peerLvlPassive,
			otgTransportMode: peerLvlPassive,
		},
		{
			name:             "Test transport mode active on ATE and passive on DUT at peer group level",
			dutConf:          bgpCreateNbr(&bgpTestParams{localAS: dutAS, peerAS: ateAS, nbrLocalAS: dutAS, transportMode: peerLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: peerLvlActive}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ESTABLISHED,
			dutTransportMode: peerLvlPassive,
			otgTransportMode: peerLvlActive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("Clear BGP configuration")
			bgpClearConfig(t, dut)

			t.Log("Configure BGP Configs on DUT")
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)
			fptest.LogQuery(t, "DUT BGP Config ", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

			t.Log("Configure BGP on ATE")
			ate.OTG().PushConfig(t, tc.ateConf)
			ate.OTG().StartProtocols(t)

			t.Logf("Verify BGP telemetry")
			verifyBgpTelemetry(t, dut, tc.wantBGPState, tc.dutTransportMode, tc.otgTransportMode)

			t.Logf("Verify BGP telemetry on otg")
			verifyOTGBGPTelemetry(t, ate.OTG(), tc.ateConf)

			t.Log("Clear BGP Configs on ATE")
			ate.OTG().StopProtocols(t)
		})
	}
}
