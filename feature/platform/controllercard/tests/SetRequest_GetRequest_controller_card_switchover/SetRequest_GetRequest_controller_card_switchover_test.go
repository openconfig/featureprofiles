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

package controller_card_switchover_config_pull_and_push_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	oc "github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	controlcardType                 = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	setRequestTimeout               = 30 * time.Second
	getRequestTimeout               = 10 * time.Second
	controllerCardSwitchoverTimeout = 2 * time.Minute
	sleepTimeBtwAttempts            = 10 * time.Second
	lastRequestTime                 = 110 * time.Second
	maxResponseTime                 = 120 * time.Second
	bgpPeerGrpName                  = "BGP-PEER-GROUP1"
	globalRouterID                  = "192.0.2.1"
	peerASN                         = 64501
	localASN                        = 65501
	IPv4PrefixLen                   = 30
	IPv6PrefixLen                   = 126
)

type activeStandByControllerCards struct {
	activeControllerCard  string
	standbyControllerCard string
}

// configParams holds the parameters for the OpenConfig configuration
type configParams struct {
	NumLAGInterfaces            int
	NumEthernetInterfacesPerLAG int
	NumBGPNeighbors             int
}

type switchoverControllerCardsConfig struct {
	controllerCards *activeStandByControllerCards
	gnoiClient      gnoigo.Clients
	requestTimeout  time.Duration
}

func fetchActiveStandbyControllerCards(t *testing.T, dut *ondatra.DUTDevice, controllerCards *[]string) activeStandByControllerCards {
	t.Helper()

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, *controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)
	return activeStandByControllerCards{rpActiveBeforeSwitch, rpStandbyBeforeSwitch}
}

func switchoverControllerCards(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, config *switchoverControllerCardsConfig) {
	t.Helper()

	// Check if active RP is ready for switchover
	controllerCards := &config.controllerCards
	switchoverReady := gnmi.OC().Component((*controllerCards).activeControllerCard).SwitchoverReady()
	switchOverReadyTimeout := 10 * time.Minute
	gnmi.Await(t, dut, switchoverReady.State(), switchOverReadyTimeout, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	// Initiate a RP switchover
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath((*controllerCards).standbyControllerCard, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)

	ctxWithTimeout, cancelWithTimeout := context.WithTimeout(ctx, config.requestTimeout)
	defer cancelWithTimeout()

	switchoverResponse, err := config.gnoiClient.System().SwitchControlProcessor(ctxWithTimeout, switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := (*controllerCards).standbyControllerCard
	var got string
	if useNameOnly {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	t.Logf("success: switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
}

func buildGetRequest(t *testing.T) *gpb.GetRequest {
	t.Helper()

	return &gpb.GetRequest{
		Prefix: &gpb.Path{},
		Path: []*gpb.Path{
			{},
		},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	}
}

var (
	numPorts int
	params   configParams
)

func setConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()

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
		if result := gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg); result.RawResponse.Message.GetCode() != 0 {
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
	if result := batch.Set(t, dut); result.RawResponse.Message.GetCode() != 0 {
		return fmt.Errorf("unable to set configuration")
	}
	return nil
}

func TestControllerCardLargeConfigPushAndPull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Get the number of ports on the DUT
	numPorts = len(dut.Ports())
	t.Logf("Number of ports on DUT: %d", numPorts)
	// Not assuming that oc base config is loaded.
	// Config the hostname to prevent the test failure when oc base config is not loaded
	gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), "ondatraHost")
	// Configuring the network instance as some devices only populate OC after configuration.
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get Controller Card list that are inserted in the DUT.
	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)
	if got, want := len(controllerCards), 2; got < want {
		t.Fatalf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	params = configParams{
		NumLAGInterfaces:            numPorts,
		NumEthernetInterfacesPerLAG: 1,
		NumBGPNeighbors:             15,
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	ctx := context.Background()
	t.Run("testLargeConfigSetRequest", func(t *testing.T) {
		testLargeConfigSetRequest(ctx, t, dut, gnoiClient, &controllerCards)
	})
	t.Run("testLargeConfigGetRequest", func(t *testing.T) {
		testLargeConfigGetRequest(ctx, t, dut, gnoiClient, &controllerCards)
	})
}

func verifyConfiguredElements(t *testing.T, dut *ondatra.DUTDevice, config *gpb.GetResponse) {
	t.Helper()

	var root oc.Root
	if len(config.GetNotification()) == 0 {
		t.Fatalf("No notification received in get response")
	}
	data := config.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
	if err := oc.Unmarshal(data, &root, &ytypes.IgnoreExtraFields{}); err != nil {
		t.Fatalf("Could not unmarshal config: %v", err)
	}
	numInterfaces := len(root.Interface)
	if numInterfaces != params.NumLAGInterfaces+numPorts {
		t.Fatalf("Number of interfaces mismatch: got: %d, want: %d", numInterfaces, params.NumLAGInterfaces+numPorts)
	}
	numBGPNeighbors := 0
	for _, networkInterface := range root.NetworkInstance {
		for _, protocol := range networkInterface.Protocol {
			if protocol.Identifier == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
				if protocol.Bgp != nil {
					numBGPNeighbors += len(protocol.Bgp.Neighbor)
				}
			}
		}
	}
	if numBGPNeighbors != 2*params.NumBGPNeighbors {
		t.Fatalf("Number of BGP neighbors mismatch: got: %d, want: %d", numBGPNeighbors, 2*params.NumBGPNeighbors)
	}
}

func testLargeConfigSetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, controllerCards *[]string) {
	activeStandbyCC := fetchActiveStandbyControllerCards(t, dut, controllerCards)
	switchoverControllerCards(ctx, t, dut, &switchoverControllerCardsConfig{&activeStandbyCC, gnoiClient, controllerCardSwitchoverTimeout})
	switchoverResponseTime := time.Now()

	var setResponseTime time.Time
	var setErr error
	for attempt := 1; attempt <= 4; attempt++ {
		if attempt > 1 {
			time.Sleep(sleepTimeBtwAttempts)
		}
		setErr = sendSetRequest(ctx, t, dut, setConfig)
		setResponseTime = time.Now()
		if setErr != nil {
			t.Logf("Error during set request on attempt %d: %v", attempt, setErr)
			if setResponseTime.Sub(switchoverResponseTime) > lastRequestTime {
				t.Fatalf("gNMI Set response after switchover time: %v, got non-zero status code", setResponseTime.Sub(switchoverResponseTime))
			}
			t.Logf("gNMI Set response after switchover time: %v, got non-zero grpc status code, retrying", setResponseTime.Sub(switchoverResponseTime))
			continue
		}
		if setResponseTime.Sub(switchoverResponseTime) > maxResponseTime {
			t.Fatalf("gNMI Set response after switchover time: %v, got SUCCESS, but exceeded max response time: %v", setResponseTime.Sub(switchoverResponseTime), maxResponseTime)
		}
		t.Logf("gNMI Set response after switchover time: %v, got SUCCESS", setResponseTime.Sub(switchoverResponseTime))
		break
	}
	if setErr != nil {
		t.Fatalf("Failed to send gNMI Set request after all attempts: %v", setErr)
	}
	// Retrieve configuration from DUT DUT using gNMI `GetRequest`.
	gnmiClient := dut.RawAPIs().GNMI(t)
	getRequest := buildGetRequest(t)

	ctxWithTimeout, cancelWithTimeout := context.WithTimeout(context.Background(), getRequestTimeout)
	defer cancelWithTimeout()
	fullConfig, err := gnmiClient.Get(ctxWithTimeout, getRequest)
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	verifyConfiguredElements(t, dut, fullConfig)
}

func testLargeConfigGetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, controllerCards *[]string) {
	activeStandbyCC := fetchActiveStandbyControllerCards(t, dut, controllerCards)
	if err := sendSetRequest(ctx, t, dut, setConfig); err != nil {
		t.Fatalf("Unable to send config to the device; err: %v", err)
	}

	gnmiClient := dut.RawAPIs().GNMI(t)
	getRequest := buildGetRequest(t)
	previousFullConfig, err := gnmiClient.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}
	// Check if the notification is empty
	if len(previousFullConfig.GetNotification()) == 0 {
		t.Fatalf("No notification received in get response")
	}
	// Check for gRPC status code
	if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
		t.Fatalf("gNMI GET response got non-zero status code: %d", st.Code())
	}

	switchoverControllerCards(ctx, t, dut, &switchoverControllerCardsConfig{&activeStandbyCC, gnoiClient, controllerCardSwitchoverTimeout})

	var currentFullConfig *gpb.GetResponse
	var getResponseTime time.Time

	// Time at the switchoverControllerCards completion
	switchoverResponseTime := time.Now()

	for attempt := 1; attempt <= 11; attempt++ {
		if attempt > 1 {
			time.Sleep(sleepTimeBtwAttempts)
		}
		ctxWithTimeout, cancelWithTimeout := context.WithTimeout(ctx, getRequestTimeout)
		defer cancelWithTimeout()
		t.Logf("Tying to get config on attempt %d", attempt)
		currentFullConfig, err = gnmiClient.Get(ctxWithTimeout, getRequest)
		getResponseTime = time.Now()
		if err != nil {
			t.Logf("Error getting config on attempt %d: %v", attempt, err)
			continue
		}
		// Check if the notification is empty
		if len(currentFullConfig.GetNotification()) == 0 {
			t.Logf("No notification received in get response on attempt %d", attempt)
			continue
		}
		// Check for gRPC status code
		if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
			if getResponseTime.Sub(switchoverResponseTime) > lastRequestTime {
				t.Fatalf("gNMI Get response after switchover time: %v, got non-zero grpc status code: %v", getResponseTime.Sub(switchoverResponseTime), st.Code())
			}
			t.Logf("gNMI Get response after switchover time: %v, got non-zero grpc status code: %v", getResponseTime.Sub(switchoverResponseTime), st.Code())
			continue
		}
		if getResponseTime.Sub(switchoverResponseTime) > maxResponseTime {
			t.Fatalf("gNMI Get response after switchover time: %v, got SUCCESS, but exceeded max response time: %v", getResponseTime.Sub(switchoverResponseTime), maxResponseTime)
		}
		t.Logf("gNMI Get response after switchover time: %v, got SUCCESS", getResponseTime.Sub(switchoverResponseTime))
		break
	}
	// Check if a successful get response was received
	if currentFullConfig == nil {
		t.Fatalf("Failed to get a successful gNMI Get response after all attempts")
	}
	// Compare the previous and current config
	sortUpdates := protocmp.SortRepeated(func(a, b *gpb.Update) bool {
		return a.String() < b.String()
	})
	if diff := cmp.Diff(previousFullConfig, currentFullConfig, protocmp.Transform(), protocmp.IgnoreFields(&gpb.Notification{}, "timestamp"), sortUpdates); diff != "" {
		t.Errorf("Configuration does not match after switchover, diff (-previous +current):\n%s", diff)
	} else {
		t.Logf("Configuration matches after switchover")
	}
}

type setRequest func(t *testing.T, dut *ondatra.DUTDevice) error

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
