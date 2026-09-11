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
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"

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
	lastRequestTime                 = 120 * time.Second
	maxResponseTime                 = 150 * time.Second
	bgpPeerGrpName                  = "BGP-PEER-GROUP1"
	globalRouterID                  = "198.18.2.1"
	peerASN                         = 64501
	localASN                        = 65501
	IPv4PrefixLen                   = 31
	IPv6PrefixLen                   = 127
	isisInstance                    = "DEFAULT"
)

type activeStandByControllerCards struct {
	activeControllerCard  string
	standbyControllerCard string
}

var aggIDs []string
var configBatch *gnmi.SetBatch
var numRE = regexp.MustCompile(`(\d+)`)

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
	switchOverReadyTimeout := 30 * time.Minute
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
			{
				Elem: []*gpb.PathElem{{Name: "interfaces"}},
			},
			{
				Elem: []*gpb.PathElem{{Name: "network-instances"}},
			},
			{
				Elem: []*gpb.PathElem{{Name: "routing-policy"}},
			},
		},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	}
}

var (
	numPorts int
	params   configParams
)

// nextAggregates is like netutil.NextAggregateInterface but obtains multiple
// aggregate interfaces.
func nextAggregates(t *testing.T, dut *ondatra.DUTDevice, n int) []string {
	firstAgg := netutil.NextAggregateInterface(t, dut)
	start, err := strconv.Atoi(numRE.FindString(firstAgg))
	if err != nil {
		t.Fatalf("Cannot extract integer from %q: %v", firstAgg, err)
	}
	aggs := []string{firstAgg}
	for i := start + 1; i < start+n; i++ {
		agg := numRE.ReplaceAllStringFunc(firstAgg, func(_ string) string {
			return strconv.Itoa(i)
		})
		// some aggregate interface after firstAgg may already be present in the system.
		_, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(agg).Name().State()).Val()
		if !present {
			aggs = append(aggs, agg)
		} else {
			n++
		}
	}
	return aggs
}

func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string, batch *gnmi.SetBatch) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}

	gnmi.BatchUpdate(batch, gnmi.OC().Config(), d)
}

func buildConfigBatch(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	configBatch = &gnmi.SetBatch{}

	// Map member ports to their respective LAG interface to support atomic bundle setups.
	lagMembers := make(map[string][]*ondatra.Port)
	ethIdx := 0
	for lagIdx := 0; ethIdx < numPorts && lagIdx < len(aggIDs); lagIdx++ {
		aggID := aggIDs[lagIdx]
		for ethAdded := 0; ethIdx < numPorts && ethAdded < params.NumEthernetInterfacesPerLAG; ethAdded++ {
			port := dut.Port(t, fmt.Sprintf("port%d", ethIdx+1))
			lagMembers[aggID] = append(lagMembers[aggID], port)
			ethIdx++
		}
	}

	if deviations.AggregateAtomicUpdate(dut) {
		for _, aggID := range aggIDs {
			ports := lagMembers[aggID]
			for _, port := range ports {
				gnmi.BatchDelete(configBatch, gnmi.OC().Interface(port.Name()).Ethernet().Config())
			}

			setupAggregateAtomically(t, dut, ports, aggID, configBatch)
		}
	}
	for i := 0; i < params.NumLAGInterfaces; i++ {
		lagInterfaceAttrs := attrs.Attributes{
			Desc:    fmt.Sprintf("LAG Interface %d", i+1),
			IPv4:    fmt.Sprintf("198.18.%d.1", i+1),
			IPv6:    fmt.Sprintf("2001:db8::%d:1", i+1),
			IPv4Len: IPv4PrefixLen,
			IPv6Len: IPv6PrefixLen,
		}
		aggID := aggIDs[i]
		t.Logf(" Inside buildConfigBatch loop i= %d , aggID is %v", i, aggID)

		agg := lagInterfaceAttrs.NewOCInterface(aggID, dut)
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		aggLag := agg.GetOrCreateAggregation()
		aggLag.LagType = oc.IfAggregate_AggregationType_STATIC

		gnmi.BatchReplace(configBatch, gnmi.OC().Interface(aggID).Config(), agg)
	}

	device := &oc.Root{}
	networkInterface := device.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	isisProto := networkInterface.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	isisProto.Enabled = ygot.Bool(true)
	isis := isisProto.GetOrCreateIsis()
	isis.GetOrCreateGlobal().Instance = ygot.String(isisInstance)
	for _, agg := range aggIDs {
		isisIntf := isis.GetOrCreateInterface(agg)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	}

	gnmi.BatchReplace(configBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config(), isisProto)

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
		bgpNbrV4 := bgp.GetOrCreateNeighbor(fmt.Sprintf("198.18.2.%d", i))
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
	gnmi.BatchReplace(configBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProto)

	ethIdx = 0
	for lagIdx := 0; ethIdx < numPorts && lagIdx < len(aggIDs); lagIdx++ {
		for ethAdded := 0; ethIdx < numPorts && ethAdded < params.NumEthernetInterfacesPerLAG; ethAdded++ {
			port := dut.Port(t, fmt.Sprintf("port%d", ethIdx+1))
			intf := device.GetOrCreateInterface(port.Name())
			intf.GetOrCreateEthernet().AggregateId = ygot.String(aggIDs[lagIdx])
			intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			if deviations.InterfaceEnabled(dut) {
				intf.Enabled = ygot.Bool(true)
			}
			gnmi.BatchReplace(configBatch, gnmi.OC().Interface(port.Name()).Config(), intf)
			ethIdx++
		}
	}
}

func setConfig(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	if result := configBatch.Set(t, dut); result.RawResponse.Message.GetCode() != 0 {
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

func verifyConfiguredElements(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// Verify Interfaces
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().Config())
	numInterfaces := len(interfaces)
	if numInterfaces < params.NumLAGInterfaces+numPorts {
		t.Fatalf("Number of interfaces mismatch: got: %d, want>= %d", numInterfaces, params.NumLAGInterfaces+numPorts)
	}

	// Verify BGP Neighbors
	dni := deviations.DefaultNetworkInstance(dut)
	bgpNeighbors := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().NeighborAny().NeighborAddress().Config())
	numBGPNeighbors := len(bgpNeighbors)

	if numBGPNeighbors != 2*params.NumBGPNeighbors {
		t.Fatalf("Number of BGP neighbors mismatch: got: %d, want: %d", numBGPNeighbors, 2*params.NumBGPNeighbors)
	}
	t.Logf("Success: Verified all the configured elements")
}

func testLargeConfigSetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, controllerCards *[]string) {
	activeStandbyCC := fetchActiveStandbyControllerCards(t, dut, controllerCards)
	// Define aggIDs
	aggIDs = nextAggregates(t, dut, params.NumLAGInterfaces)
	t.Logf("****nextAggregates generated aggIDS is %s ", aggIDs)
	buildConfigBatch(t, dut)
	switchoverControllerCards(ctx, t, dut, &switchoverControllerCardsConfig{&activeStandbyCC, gnoiClient, controllerCardSwitchoverTimeout})
	switchoverResponseTime := time.Now()

	// Wait for the gNMI agent to be responsive
	{
		gnmiClient := dut.RawAPIs().GNMI(t)
		getRequest := buildGetRequest(t)
		var err error
		for attempt := 1; attempt <= 12; attempt++ {
			if attempt > 1 {
				time.Sleep(sleepTimeBtwAttempts)
			}
			ctxWithTimeout, cancelWithTimeout := context.WithTimeout(ctx, getRequestTimeout)
			_, err = gnmiClient.Get(ctxWithTimeout, getRequest)
			cancelWithTimeout()
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Fatalf("gNMI agent did not become responsive: %v", err)
		}
	}

	var setResponseTime time.Time
	var setErr error
	for attempt := 1; attempt <= 4; attempt++ {
		if attempt > 1 {
			time.Sleep(sleepTimeBtwAttempts)
		}
		setErr = sendSetRequest(ctx, t, dut, setConfig)
		setResponseTime = time.Now()
		if setErr != nil {
			t.Logf("Log: Error during set request on attempt %d: %v", attempt, setErr)
			if setResponseTime.Sub(switchoverResponseTime) > lastRequestTime {
				t.Fatalf("gNMI Set response after switchover time: %v, got non-zero status code", setResponseTime.Sub(switchoverResponseTime))
			}
			t.Logf("gNMI Set response after switchover time: %v, got non-zero grpc status code, retrying", setResponseTime.Sub(switchoverResponseTime))
			continue
		}
		if setResponseTime.Sub(switchoverResponseTime) > maxResponseTime {
			t.Fatalf("gNMI Set response after switchover time: %v, got SUCCESS, but exceeded max response time: %v", setResponseTime.Sub(switchoverResponseTime), maxResponseTime)
		}
		t.Logf("****SUCESS!!!gNMI Set response after switchover time: %v, got SUCCESS in attempt %d", setResponseTime.Sub(switchoverResponseTime), attempt)
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
	_, err := gnmiClient.Get(ctxWithTimeout, getRequest)
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	verifyConfiguredElements(t, dut)
	t.Logf("**Passed verifyConfiguredElements ***")
}
func testLargeConfigGetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnoiClient gnoigo.Clients, controllerCards *[]string) {
	activeStandbyCC := fetchActiveStandbyControllerCards(t, dut, controllerCards)

	t.Log("Verifying configuration counts BEFORE switchover...")
	verifyConfiguredElements(t, dut)

	// Trigger the Stateful Switchover
	switchoverControllerCards(ctx, t, dut, &switchoverControllerCardsConfig{&activeStandbyCC, gnoiClient, controllerCardSwitchoverTimeout})

	// Wait for the gNMI agent to become responsive again after the switchover
	gnmiClient := dut.RawAPIs().GNMI(t)
	getRequest := buildGetRequest(t)
	var err error

	for attempt := 1; attempt <= 11; attempt++ {
		if attempt > 1 {
			time.Sleep(sleepTimeBtwAttempts)
		}
		ctxWithTimeout, cancelWithTimeout := context.WithTimeout(ctx, getRequestTimeout)
		t.Logf("Checking if gNMI is responsive on attempt %d", attempt)
		_, err = gnmiClient.Get(ctxWithTimeout, getRequest)
		cancelWithTimeout()
		if err == nil {
			t.Logf("gNMI agent is responsive!")
			break
		}
	}

	if err != nil {
		t.Fatalf("gNMI agent did not become responsive after switchover: %v", err)
	}

	t.Log("Verifying configuration counts AFTER switchover...")
	// If the config was lost or corrupted, this function will fail the test
	verifyConfiguredElements(t, dut)

	t.Logf("Successfully verified all Interfaces and BGP neighbors survived the switchover perfectly.")
}

type setRequest func(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) error

func sendSetRequest(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, set setRequest) error {
	t.Helper()

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, setRequestTimeout)
	defer cancelTimeout()

	return set(ctxTimeout, t, dut)
}
