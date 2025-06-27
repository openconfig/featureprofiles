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

package chassis_reboot_status_and_cancel_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	oneMinuteInNanoSecond = 6e10
	rebootDelay           = 120
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Send gNOI reboot status request.
//   - Check the reboot status before sending reboot request.
//     - Verify the reboot status is not active.
//   - Check the reboot status after sending reboot request.
//     - Verify the reboot status is active.
//     - Verify the reason from reboot status response matches reboot message.
//     - Verify the wait time from reboot status response matches reboot delay.
//  2) Cancel gNOI reboot request.
//   - Cancel reboot request before the test
//     - Verify that there is no response error returned.
//   - Send reboot request with delay.
//     - Verify the reboot status is active.
//   - Send reboot cancel request.
//     - Verify the reboot status is not active.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestRebootStatus(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)

	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
		rebootActive  bool
		cancelReboot  bool
	}{
		{
			desc:          "no reboot requested",
			rebootRequest: nil,
			rebootActive:  false,
		},
		{
			desc: "reboot requested with delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   rebootDelay * oneMinuteInNanoSecond,
				Message: "Reboot chassis with delay",
				Force:   true,
			},
			rebootActive: true,
		},
	}

	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.rebootRequest != nil {
				t.Logf("Send reboot request: %v", tc.rebootRequest)
				rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
				defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
				t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
				if err != nil {
					t.Fatalf("Failed to request reboot with unexpected err: %v", err)
				}
			}
			resp, err := gnoiClient.System().RebootStatus(context.Background(), statusReq)
			t.Logf("DUT rebootStatus: %v, err: %v", resp, err)
			if err != nil {
				t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
			}
			if resp.GetActive() != tc.rebootActive {
				t.Errorf("resp.GetActive(): got %v, want %v", resp.GetActive(), tc.rebootActive)
			}

			if tc.rebootRequest != nil {
				if resp.GetReason() != tc.rebootRequest.GetMessage() {
					t.Errorf("resp.GetReason(): got %v, want %v", resp.GetReason(), tc.rebootRequest.GetMessage())
				}
				if resp.GetWait() > tc.rebootRequest.GetDelay() {
					t.Errorf("resp.GetWait(): got %v, want <= %v", resp.GetWait(), tc.rebootRequest.GetDelay())
				}
				if resp.GetWhen() == 0 {
					t.Errorf("resp.GetWhen(): got %v, want > 0", resp.GetWhen())
				}
			}
		})

		t.Logf("Cancel reboot request after the test")

		rebootCancel, err := gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
		if err != nil {
			t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
		}
		t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)
	}
}

func TestCancelReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)

	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   rebootDelay * oneMinuteInNanoSecond,
		Message: "Reboot chassis with delay",
		Force:   true,
	}

	t.Logf("Cancel reboot request before the test")
	rebootCancel, err := gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	if err != nil {
		t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
	}
	t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)

	t.Logf("Send reboot request: %v", rebootRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to request reboot with unexpected err: %v", err)
	}
	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
	}
	rebootStatus, err := gnoiClient.System().RebootStatus(context.Background(), statusReq)
	t.Logf("DUT rebootStatus: %v, err: %v", rebootStatus, err)
	if err != nil {
		t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
	}
	if !rebootStatus.GetActive() {
		t.Errorf("rebootStatus.GetActive(): got %v, want true", rebootStatus.GetActive())
	}

	t.Logf("Cancel reboot request: %v", rebootRequest)
	rebootCancel, err = gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
	t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)
	if err != nil {
		t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
	}

	rebootStatus, err = gnoiClient.System().RebootStatus(context.Background(), statusReq)
	t.Logf("DUT rebootStatus: %v, err: %v", rebootStatus, err)
	if err != nil {
		t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
	}
	if rebootStatus.GetActive() {
		t.Errorf("rebootStatus.GetActive(): got %v, want false", rebootStatus.GetActive())
	}
}

func TestRebootPlusConfigPush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	LargeConfigPush(t)
	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
		rebootActive  bool
		cancelReboot  bool
	}{
		{
			desc: "reboot requested without delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   0,
				Message: "Reboot chassis without delay",
				Force:   true,
			},
			rebootActive: true,
		},
	}

	statusReq := &spb.RebootStatusRequest{Subcomponents: []*tpb.Path{}}
	if !deviations.GNOIStatusWithEmptySubcomponent(dut) {
		statusReq.Subcomponents = append(statusReq.Subcomponents, getSubCompPath(t, dut))
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.rebootRequest != nil {
				t.Logf("Send reboot request: %v", tc.rebootRequest)
				rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
				defer gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
				t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
				if err != nil {
					t.Fatalf("Failed to request reboot with unexpected err: %v", err)
				}
			}
			resp, err := gnoiClient.System().RebootStatus(context.Background(), statusReq)
			t.Logf("DUT rebootStatus: %v, err: %v", resp, err)
			if err != nil {
				t.Fatalf("Failed to get reboot status with unexpected err: %v", err)
			}
			if resp.GetActive() != tc.rebootActive {
				t.Errorf("resp.GetActive(): got %v, want %v", resp.GetActive(), tc.rebootActive)
			}

			if tc.rebootRequest != nil {
				if resp.GetReason() != tc.rebootRequest.GetMessage() {
					t.Errorf("resp.GetReason(): got %v, want %v", resp.GetReason(), tc.rebootRequest.GetMessage())
				}
				if resp.GetWait() > tc.rebootRequest.GetDelay() {
					t.Errorf("resp.GetWait(): got %v, want <= %v", resp.GetWait(), tc.rebootRequest.GetDelay())
				}
				if resp.GetWhen() == 0 {
					t.Errorf("resp.GetWhen(): got %v, want > 0", resp.GetWhen())
				}
			}
		})

		t.Logf("Cancel reboot request after the test")

		rebootCancel, err := gnoiClient.System().CancelReboot(context.Background(), &spb.CancelRebootRequest{})
		if err != nil {
			t.Fatalf("Failed to cancel reboot with unexpected err: %v", err)
		}
		t.Logf("DUT CancelReboot response: %v, err: %v", rebootCancel, err)
	}
}

func LargeConfigPush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Get the number of ports on the DUT
	numPorts := len(dut.Ports())
	t.Logf("Number of ports on DUT: %d", numPorts)
	// Not assuming that oc base config is loaded.
	// Config the hostname to prevent the test failure when oc base config is not loaded
	gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), "ondatraHost")
	// Configuring the network instance as some devices only populate OC after configuration.
	fptest.ConfigureDefaultNetworkInstance(t, dut)	
	ctx := context.Background()
	t.Run("testLargeConfigSetRequest", func(t *testing.T) {
		_ = sendSetRequest(ctx, t, dut, setConfig)
	})
}

func setConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	params := configParams{
		NumLAGInterfaces:            numPorts,
		NumEthernetInterfacesPerLAG: 1,
		NumBGPNeighbors:             15,
	}
	var aggIDs []string
	for i := 1; i <= params.NumLAGInterfaces; i++ {
		lagInterfaceAttrs := attrs.Attributes{
			Desc:    fmt.Sprintf("LAG Interface %d", i),
			IPv4:    "192.0.2.5",
			IPv6:    "2001:db8::5",
			IPv4Len: IPv4PrefixLen,
			IPv6Len: IPv6PrefixLen,
		}
		aggID := netutil.NextAggregateInterface(t, dut)

		aggIDs = append(aggIDs, aggID)
		agg := lagInterfaceAttrs.NewOCInterface(aggID, dut)
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
		if err := gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg); err != nil {
			return fmt.Errorf("unable to set lag interface")
		}
	}

	batch := &gnmi.SetBatch{}
	device := &oc.Root{}

	networkInterface := device.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	isisProto := networkInterface.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "ISIS")
	isisProto.Enabled = ygot.Bool(true)
	isis := isisProto.GetOrCreateIsis()
	for _, agg := range aggIDs {
		isisIntf := isis.GetOrCreateInterface(agg)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	}
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "ISIS").Config(), isisProto)

	bgpProto := networkInterface.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(globalRouterID)
	global.As = ygot.Uint32(localASN)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(bgpPeerGrpName)
	pg.PeerAs = ygot.Uint32(peerASN)
	pg.PeerGroupName = ygot.String(bgpPeerGrpName)

	for i := 5; i < params.NumBGPNeighbors+5; i++ {
		bgpNbrV4 := bgp.GetOrCreateNeighbor(fmt.Sprintf("192.0.2.%d", i))
		bgpNbrV4.PeerGroup = ygot.String(bgpPeerGrpName)
		bgpNbrV4.PeerAs = ygot.Uint32(peerASN)
		bgpNbrV4.Enabled = ygot.Bool(true)
		af4 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)

		bgpNbrV6 := bgp.GetOrCreateNeighbor(fmt.Sprintf("2001:db8::%d", i))
		bgpNbrV6.PeerGroup = ygot.String(bgpPeerGrpName)
		bgpNbrV6.PeerAs = ygot.Uint32(peerASN)
		bgpNbrV6.Enabled = ygot.Bool(true)
		af4 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(false)
		af6 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProto)

	ethIdx := 0
	for lagIdx := 0; ethIdx < numPorts && lagIdx < len(aggIDs); lagIdx++ {
		for ethAdded := 0; ethIdx < numPorts && ethAdded < params.NumEthernetInterfacesPerLAG; ethAdded++ {
			port := dut.Port(t, fmt.Sprintf("port%d", ethIdx+1))
			intf := device.GetOrCreateInterface(port.Name())
			intf.GetOrCreateEthernet().AggregateId = ygot.String(aggIDs[lagIdx])
			intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			if deviations.InterfaceEnabled(dut) {
				intf.Enabled = ygot.Bool(true)
			}
			gnmi.BatchReplace(batch, gnmi.OC().Interface(port.Name()).Config(), intf)
			ethIdx++
		}
	}
	if err := batch.Set(t, dut); err != nil {
		return fmt.Errorf("unable to set configuration")
	}
	return nil
}

func sendSetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, set setRequest) error {
	t.Helper()

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, setRequestTimeout)
	defer cancelTimeout()

	done := make(chan error, 1)

	go func() {
		err := set(t, dut)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-ctxTimeout.Done():
		return ctxTimeout.Err()
	}
}

