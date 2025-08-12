// Copyright 2024 Google LLC
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

package ipv4_guev1_decap_and_hashing_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// Constants for BGP ASNs
const (
	dutAS                = 100
	ate1AS               = 200   // EBGP with DUT Port1
	ate2AS               = dutAS // IBGP with DUT Port2
	ate3AS               = dutAS // IBGP with DUT LAG1
	ate4AS               = 200   // EBGP with DUT LAG2
	ate5AS               = 200   // EBGP with DUT Port7
	plenIPv4             = 30
	plenIPv6             = 126
	advertisedIPv4PfxLen = 24
	advertisedIPv6PfxLen = 64
	loopbackPfxLen       = 32
	isisInstance         = "DEFAULT"
	dutAreaAddress       = "49.0001"
	dutSysID             = "1920.0000.2001"
	ateSysID             = "64000000000"
	UdpSrcPort           = 5996
	UdpDstPort           = 6080
	UdpDstPortNeg        = 6085
	testSrcPort          = 14
	testDstPort          = 15
	flowCount            = 10 // Number of prefixes/routes per host group
	dcapIp               = "192.168.0.1"
	tolerance            = 5 // As per readme, Tolerance for delta: 5%
	fixedPackets         = 1000000
	trafficFrameSize     = 1500
	ratePercent          = 10
	lspV4Name            = "lsp-egress-v4"
	rplName              = "ALLOW"
	mplsLabel            = 1000
	decapType            = "udp"
	decapPort            = 6080
	ecmpMaxPath          = 4
	policyName           = "decap-policy"
	policyId             = 1
	trafficDuration      = 20
)

// IP Addresses and Attributes
var (
	// DUT Loopback0 (GUE Decap Address)
	dutLo0 = attrs.Attributes{Desc: "DUT Loopback0", IPv4: "192.168.3.2", IPv4Len: loopbackPfxLen, IPv6: "2001:db8:c000::1", IPv6Len: 128}

	// DUT Port1 <> ATE Port1 (ATE1)
	dutP1 = attrs.Attributes{Desc: "DUT Port1", IPv4: "192.0.1.1", IPv6: "2001:db8:1::1", MAC: "02:00:01:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP1 = attrs.Attributes{Name: "ateP1", IPv4: "192.0.1.2", IPv6: "2001:db8:1::2", MAC: "02:00:01:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// DUT Port2 <> ATE Port2 (ATE2)
	dutP2 = attrs.Attributes{Desc: "DUT Port2", IPv4: "192.0.2.1", IPv6: "2001:db8:2::1", MAC: "02:00:02:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP2 = attrs.Attributes{Name: "ateP2", IPv4: "192.0.2.2", IPv6: "2001:db8:2::2", MAC: "02:00:02:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// DUT LAG (Port3, Port4) <> ATE LAG (Port3, Port4) (ATE3)
	dutLag1 = attrs.Attributes{Desc: "DUTLag1", IPv4: "192.0.3.1", IPv6: "2001:db8:3::1", MAC: "02:00:03:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag1 = attrs.Attributes{Name: "ateLag1", IPv4: "192.0.3.2", IPv6: "2001:db8:3::2", MAC: "02:00:03:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	dutLag2 = attrs.Attributes{Desc: "DUTLag2", IPv4: "192.0.4.1", IPv6: "2001:db8:4::1", MAC: "02:00:04:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag2 = attrs.Attributes{Name: "ateLag2", IPv4: "192.0.4.2", IPv6: "2001:db8:4::2", MAC: "02:00:04:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// ATE3 Loopback (for ISIS passive demo)
	ate3Lo = attrs.Attributes{Name: "ate3Lo0", IPv4: "192.168.3.1", IPv6: "2001:db8:10::1", IPv4Len: loopbackPfxLen, IPv6Len: 128}

	// DUT Port7 <--> ATE P7 (Represents ATE5 in diagram)
	dutP7 = attrs.Attributes{Desc: "DUT Port7", IPv4: "192.0.7.1", IPv6: "2001:db8:7::1", MAC: "02:00:05:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP7 = attrs.Attributes{Name: "atep7", IPv4: "192.0.7.2", IPv6: "2001:db8:7::2", MAC: "02:00:05:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	// Advertised Prefixes (base addresses)
	host1IPv4Start         = "198.51.100.0"
	host1IPv6Start         = "2001:db8:100::"
	host2IPv4Start         = "198.51.110.0"
	host2IPv6Start         = "2001:db8:110::"
	host3IPv4Start         = "198.51.120.0"
	host3IPv6Start         = "2001:db8:120::"
	host4IPv4Start         = "198.51.130.0"
	host4IPv6Start         = "2001:db8:130::"
	ate1LoopbackIP         = "172.16.1.0"
	timeout                = 1 * time.Minute
	lagTrafficDistribution = []uint64{50, 50}
	aggID1                 = "Port-Channel1"
	aggID2                 = "Port-Channel2"
)

type Neighbor struct {
	IPv4 string
	IPv6 string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMultipathGUE(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggIDs := configureDUT(t, dut)
	otgConfig := configureATE(t, ate)
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.MPLSStaticLSP(t, sfBatch, dut, lspV4Name, mplsLabel, ateLag1.IPv4, "", "ipv4")
	sfBatch.Set(t, dut)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv6")
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
	}
	neighbors := []Neighbor{
		{IPv4: ateP1.IPv4, IPv6: ateP1.IPv6},
		{IPv4: ateP2.IPv4, IPv6: ateP2.IPv6},
		{IPv4: ateLag1.IPv4, IPv6: ateLag1.IPv6},
		{IPv4: ateLag2.IPv4, IPv6: ateLag2.IPv6},
		{IPv4: ateP7.IPv4, IPv6: ateP7.IPv6},
	}
	checkBgpStatus(t, dut, neighbors)
	t.Run("PF-1.22.1[Baseline]: GUE Decapsulation over ipv4 decap address and Load-balance test", func(t *testing.T) {

		destinations := [][]string{
			{otgConfig.Lags().Items()[0].Name()},                                      // Flow#1 to H3 via ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#2 to H2 via ATE2 + ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#3 same as Flow#2
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#4 to H4 via ATE4 LAG + ATE5
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#5 same as Flow#4
			{otgConfig.Lags().Items()[0].Name()},                                      // Flow#6 to H3 via ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#7 to H2 via ATE2 + ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#8 same as Flow#7
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#9 to H4 via ATE4 LAG + ATE5
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#10 same as Flow#9
		}

		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		for flowIndex := 1; flowIndex <= 10; flowIndex++ {
			otgConfig.Flows().Clear()
			flow := configureFlows(t, otgConfig, macAddress, destinations[flowIndex-1], flowIndex, false)
			ate.OTG().PushConfig(t, otgConfig)
			ate.OTG().StartProtocols(t)
			t.Logf("Running test for flow index %d: %s", flowIndex, flow.Name())
			var payloadType, excludeLag, rxPort string
			var rxLags []string

			switch flowIndex {
			case 1, 6:
				payloadType = "mpls"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = ""
				excludeLag = otgConfig.Lags().Items()[1].Name()
			case 2, 3:
				payloadType = "ipv4"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = otgConfig.Ports().Items()[1].Name()
				excludeLag = otgConfig.Lags().Items()[1].Name()
			case 4, 5:
				payloadType = "ipv4"
				rxLags = []string{otgConfig.Lags().Items()[1].Name()}
				rxPort = otgConfig.Ports().Items()[2].Name()
				excludeLag = otgConfig.Lags().Items()[0].Name()
			case 7, 8:
				payloadType = "ipv6"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = otgConfig.Ports().Items()[1].Name()
				excludeLag = otgConfig.Lags().Items()[1].Name()
			default:
				payloadType = "ipv6"
				rxLags = []string{otgConfig.Lags().Items()[1].Name()}
				rxPort = otgConfig.Ports().Items()[2].Name()
				excludeLag = otgConfig.Lags().Items()[0].Name()
			}
			// Configure decap on DUT for current payload
			configureDutWithGueDecap(t, dut, payloadType)

			ate.OTG().StartTraffic(t)
			time.Sleep(trafficDuration * time.Second)
			ate.OTG().StopTraffic(t)
			if ok := verifyFlowTraffic(t, ate, otgConfig, flow.Name()); !ok {
				t.Fatalf("Packet loss detected in flow: %s", flow.Name())
			}
			// Validate load balancing weights
			weights := testLoadBalance(t, ate, rxLags, flow, excludeLag)
			getPortRxPkts(t, ate, flow, rxPort)
			for idx, weight := range lagTrafficDistribution {
				if got, want := weights[idx], weight; got < (want-tolerance) || got > (want+tolerance) {
					t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
				}
			}
			t.Logf("Load balancing has been verified on the LAG interfaces.")
		}
	})
	t.Run("PF-1.22.2: GUE Decapsulation over non-matching ipv4 decap address [Negative] test", func(t *testing.T) {
		var flows []gosnappi.Flow
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		for flowIndex := 11; flowIndex <= 12; flowIndex++ {
			flow := configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, false)
			flows = append(flows, flow)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		for _, flow := range flows {
			if ok := verifyFlowTraffic(t, ate, otgConfig, flow.Name()); !ok {
				t.Fatalf("Packet loss detected in flow: %s", flow.Name())
			} else {
				t.Logf("Flow %s: Traffic validation sucess", flow.Name())
			}
		}
	})
	t.Run("PF-1.22.3: GUE Decapsulation over non-matching UDP decap port [Negative] test", func(t *testing.T) {
		var flows []gosnappi.Flow
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		for flowIndex := 13; flowIndex <= 14; flowIndex++ {
			flow := configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, false)
			flows = append(flows, flow)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		for _, flow := range flows {
			if ok := verifyTrafficFlowNegCase(t, ate, otgConfig, flow); !ok {
				t.Logf("Flow %s: Packets dropped, Test Passed", flow.Name())
			} else {
				t.Fatalf("Flow %s: Packets not dropped, Test Failed", flow.Name())
			}
		}
	})
	t.Run("PF-1.22.4: Verify the Immediate next header's L4 fields are not considered in Load-Balancing Algorithm test", func(t *testing.T) {
		t.Log("Starting test: Verify that immediate next header's L4 fields are NOT used in load-balancing")
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		// Generate flows with randomized L4 ports immediately after outer header
		for flowIndex := 7; flowIndex <= 14; flowIndex++ {
			configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, true)
		}

		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)

		// Verify: Traffic should NOT be load-balanced → All traffic should go to a single port
		verifySinglePathTraffic(t, ate, otgConfig)
	})
	t.Run("PF-1.22.5: Verify the Immediate next header's L3 fields are not considered in Load-Balancing Algorithm test", func(t *testing.T) {
		t.Log("Starting test: Verify that immediate next header's L4 fields are NOT used in load-balancing")
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		// Generate flows with Immediate next header's L3 fields
		for flowIndex := 1; flowIndex <= 14; flowIndex++ {
			configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, true)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)

		// Verify: Traffic should NOT be load-balanced → All traffic should go to a single port
		verifySinglePathTraffic(t, ate, otgConfig)
	})

}

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p7 := dut.Port(t, "port7")
	var aggIDsList []string

	// Interface configurations
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutP2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p7.Name()).Config(), dutP7.NewOCInterface(p7.Name(), dut))

	// Loopback0 for GUE Decap and Router ID
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutLo0.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutLo0.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutLo0.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutLo0.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutLo0.IPv6)
	}
	// Ports 3 and 4 will be part of LAG
	dutAggPorts1 := []*ondatra.Port{
		dut.Port(t, "port3"),
		dut.Port(t, "port4"),
	}
	aggIDsList = append(aggIDsList, aggID1)
	clearLAGInterfaces(t, dut, dutAggPorts1, aggID1)
	configureDUTLag(t, dut, dutAggPorts1, aggID1, dutLag1)

	// Ports 5 and 6 will be part of LAG
	dutAggPorts2 := []*ondatra.Port{
		dut.Port(t, "port5"),
		dut.Port(t, "port6"),
	}
	aggIDsList = append(aggIDsList, aggID2)
	configureDUTLag(t, dut, dutAggPorts2, aggID2, dutLag2)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// ISIS Configuration
	dutConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	dutConf := configureDUTISIS(t, dut, p1, p2, aggID1)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	dutBgpConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	// Create BGP config only once
	dutBgpConf := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
		Bgp:        &oc.NetworkInstance_Protocol_Bgp{},
	}

	bgp := dutBgpConf.Bgp
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(dutP1.IPv4)

	af4 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	// Append EBGP Neighbors - ATE1 and ATE5
	appendBGPNeighbor(t, bgp, ate1AS, dutP1.Name, ateP1.IPv4, ateP1.IPv6, false)
	appendBGPNeighbor(t, bgp, ate5AS, dutP7.Name, ateP7.IPv4, ateP7.IPv6, false)
	// Append IBGP Neighbor - ATE2
	appendBGPNeighbor(t, bgp, ate2AS, dutP2.Name, ateP2.IPv4, ateP2.IPv6, false)
	appendBGPNeighbor(t, bgp, ate3AS, dutLag1.Name, ateLag1.IPv4, ateLag1.IPv6, true)
	appendBGPNeighbor(t, bgp, ate4AS, dutLag2.Name, ateLag2.IPv4, ateLag2.IPv6, true)

	// Apply the whole BGP config
	gnmi.Replace(t, dut, dutBgpConfPath.Config(), dutBgpConf)
	if deviations.LoadIntervalNotSupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		jsonConfig := `
		load-balance policies
		load-balance sand profile default
		packet-type gue outer-ip		
		`
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	} else {
		// TODO: OC does not yet support selecting the load-balancing hash mode on LAG members.
		t.Logf("Load balancing is currently not supported via OpenConfig. Will fix once it's implemented.")
	}

	if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
		t.Log("Executing CLI commands")
		gnmiClient := dut.RawAPIs().GNMI(t)
		jsonConfig := fmt.Sprintf(`		
		router bgp %d
		address-family ipv4 labeled-unicast
		maximum-paths %[2]d ecmp %[2]d
		bgp bestpath as-path multipath-relax
		address-family ipv6 labeled-unicast
		maximum-paths %[2]d ecmp %[2]d
		bgp bestpath as-path multipath-relax
		`, dutAS, ecmpMaxPath)
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	} else {
		// TODO: As per the latest OpenConfig GNMI OC schema — the Encapsulation/Decapsulation sub-tree is not fully implemented, need to add OC commands once implemented.
		af4.GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
		af4.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().AllowMultipleAs = ygot.Bool(true)
		af6.GetOrCreateUseMultiplePaths().Enabled = ygot.Bool(true)
		af6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().AllowMultipleAs = ygot.Bool(true)
		af4.GetOrCreateUseMultiplePaths().GetOrCreateIbgp().SetMaximumPaths(ecmpMaxPath)
		af6.GetOrCreateUseMultiplePaths().GetOrCreateIbgp().SetMaximumPaths(ecmpMaxPath)
	}
	return aggIDsList
}

func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, ipType string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", decapPort)
	ocPFParams := GetDefaultOcPolicyForwardingParams(t, dut, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            dcapIp,
		GuePort:             uint32(decapPort),
		IpType:              ipType,
		Dynamic:             true,
	}
}

func configureDUTLag(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string, dutLag attrs.Attributes) {
	t.Helper()
	for _, port := range aggPorts {
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().Config())
	}
	setupAggregateAtomically(t, dut, aggPorts, aggID)
	agg := dutLag.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)
	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
}

func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
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
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func clearLAGInterfaces(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	// Clear port bindings first
	for _, port := range aggPorts {
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Config())
	}

	// Then delete the aggregate interface itself
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Config())
}

func configureDUTISIS(t *testing.T, dut *ondatra.DUTDevice, p1, p2 *ondatra.Port, aggID string) *oc.NetworkInstance_Protocol {
	t.Helper()
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	// Set Protocol Config (no GetOrCreateConfig so use this)
	protocol := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	protocol.Enabled = ygot.Bool(true)

	isis := protocol.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance) // must match the protocol 'name'
	}
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2

	t.Logf("Enable ISIS on these interfaces %s, %s, %s", p1.Name(), p2.Name(), aggID1)
	for _, intf := range []string{p1.Name(), p2.Name(), aggID} {
		isisIf := isis.GetOrCreateInterface(intf)
		isisIf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIf.Enabled = ygot.Bool(true)
	}
	// Loopback passive
	isisLo := isis.GetOrCreateInterface("Loopback0")
	isisLo.Enabled = ygot.Bool(true)
	isisLo.Passive = ygot.Bool(true)

	return protocol
}

func appendBGPNeighbor(t *testing.T, bgp *oc.NetworkInstance_Protocol_Bgp, ateAs uint32, portName, neighborIpV4, neighborIpV6 string, isLag bool) {
	t.Helper()
	// Peer Group for IPv4
	pgv4 := bgp.GetOrCreatePeerGroup(portName + "BGP-PEER-GROUP-V4")
	pgv4.PeerAs = ygot.Uint32(ateAs)
	pgv4.PeerGroupName = ygot.String(portName + "BGP-PEER-GROUP-V4")
	pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgafv4.Enabled = ygot.Bool(true)
	rpl4 := pgafv4.GetOrCreateApplyPolicy()
	rpl4.ImportPolicy = []string{rplName}
	rpl4.ExportPolicy = []string{rplName}

	// Peer Group for IPv6
	pgv6 := bgp.GetOrCreatePeerGroup(portName + "BGP-PEER-GROUP-V6")
	pgv6.PeerAs = ygot.Uint32(ateAs)
	pgv6.PeerGroupName = ygot.String(portName + "BGP-PEER-GROUP-V6")
	pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgafv6.Enabled = ygot.Bool(true)
	rpl6 := pgafv6.GetOrCreateApplyPolicy()
	rpl6.ImportPolicy = []string{rplName}
	rpl6.ExportPolicy = []string{rplName}

	// IPv4 Neighbor
	nv4 := bgp.GetOrCreateNeighbor(neighborIpV4)
	nv4.PeerAs = ygot.Uint32(ateAs)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(portName + "BGP-PEER-GROUP-V4")
	afisafi4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afisafi4.Enabled = ygot.Bool(true)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)

	// IPv6 Neighbor
	nv6 := bgp.GetOrCreateNeighbor(neighborIpV6)
	nv6.PeerAs = ygot.Uint32(ateAs)
	nv6.Enabled = ygot.Bool(true)
	// Enable multihop on LAGs
	if isLag {
		nv4.GetOrCreateEbgpMultihop().SetMultihopTtl(5)
		nv6.GetOrCreateEbgpMultihop().SetMultihopTtl(5)
	}
	nv6.PeerGroup = ygot.String(portName + "BGP-PEER-GROUP-V6")
	afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afisafi6.Enabled = ygot.Bool(true)
	nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// Create ATE Ports
	ate1p1 := ate.Port(t, "port1")
	ate2p1 := ate.Port(t, "port2")
	ate5p7 := ate.Port(t, "port7")

	// First, define OTG ports
	ate1Port := ateConfig.Ports().Add().SetName(ate1p1.ID())
	ate2Port := ateConfig.Ports().Add().SetName(ate2p1.ID())
	ate5Port := ateConfig.Ports().Add().SetName(ate5p7.ID())
	// ATE Device 1 (EBGP)
	configureATEDevice(t, ateConfig, ate1Port, ateP1, dutP1, ate1AS, true, host1IPv4Start, host1IPv6Start, true, ate1LoopbackIP, loopbackPfxLen, true, ateSysID+"1")

	// ATE Device 2 (IBGP)
	configureATEDevice(t, ateConfig, ate2Port, ateP2, dutP2, ate2AS, false, host2IPv4Start, host2IPv6Start, false, ate1LoopbackIP, loopbackPfxLen, true, ateSysID+"2")

	// ATE LAG1 (IBGP)
	ateAggPorts1 := []*ondatra.Port{
		ate.Port(t, "port3"),
		ate.Port(t, "port4"),
	}
	configureLAGDevice(t, ateConfig, "lag1", 1, ateLag1, dutLag1, ateAggPorts1, ate3AS, false, true, host2IPv4Start, host2IPv6Start, host3IPv4Start)

	// ATE LAG2 (EBGP)
	ateAggPorts2 := []*ondatra.Port{
		ate.Port(t, "port5"),
		ate.Port(t, "port6"),
	}
	configureLAGDevice(t, ateConfig, "lag2", 2, ateLag2, dutLag2, ateAggPorts2, ate4AS, true, false, host4IPv4Start, host4IPv6Start, "")
	configureATEDevice(t, ateConfig, ate5Port, ateP7, dutP7, ate5AS, true, host4IPv4Start, host4IPv6Start, false, ate1LoopbackIP, loopbackPfxLen, false, ateSysID+"2")
	return ateConfig
}

func configureATEDevice(t *testing.T,
	cfg gosnappi.Config,
	port gosnappi.Port,
	atePort, dutPort attrs.Attributes,
	asn uint32,
	isEBGP bool,
	hostPrefixV4, hostPrefixV6 string,
	loopbacks bool,
	loopbackPrefix string,
	loopbackPrefixLen uint32,
	isisConfig bool,
	sysID string,
) {
	t.Helper()
	var peerTypeV4 gosnappi.BgpV4PeerAsTypeEnum
	var peerTypeV6 gosnappi.BgpV6PeerAsTypeEnum

	dev := cfg.Devices().Add().SetName(atePort.Name)
	eth := dev.Ethernets().Add().SetName(atePort.Name + "Eth").SetMac(atePort.MAC)
	eth.Connection().SetPortName(port.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(atePort.Name + ".IPv4")
	ip4.SetAddress(atePort.IPv4).SetGateway(dutPort.IPv4).SetPrefix(uint32(atePort.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(atePort.Name + ".IPv6")
	ip6.SetAddress(atePort.IPv6).SetGateway(dutPort.IPv6).SetPrefix(uint32(atePort.IPv6Len))

	bgp := dev.Bgp().SetRouterId(atePort.IPv4)
	if isEBGP {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP
	} else {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.IBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.IBGP
	}

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(atePort.Name + ".BGPv4.Peer").SetPeerAddress(dutPort.IPv4).SetAsNumber(asn).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(atePort.Name + ".BGPv6.Peer").SetPeerAddress(dutPort.IPv6).SetAsNumber(asn).SetAsType(peerTypeV6)

	// Advertise host routes
	addBGPRoutes(v4Peer.V4Routes().Add(), atePort.Name+".Host.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), atePort.Name+".Host.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ip6.Address())

	if loopbacks {
		addBGPRoutes(v4Peer.V4Routes().Add(), atePort.Name+".Loopbacks.v4", loopbackPrefix, loopbackPrefixLen, flowCount, ip4.Address())
	}
	if isisConfig {
		configureISIS(dev,
			ip4.Address(),
			eth.Name(),
			[]string{atePort.IPv4 + "/" + strconv.Itoa(plenIPv4)},
			[]string{atePort.IPv6 + "/" + strconv.Itoa(plenIPv6)},
			sysID,
		)
	}
}

func configureLAGDevice(t *testing.T, ateConfig gosnappi.Config, lagName string, lagID uint32, lagAttrs attrs.Attributes, dutAttrs attrs.Attributes, atePorts []*ondatra.Port, asn uint32, isEBGP, isISIS bool, hostPrefixV4, hostPrefixV6, host3PrefixV4 string) {
	t.Helper()
	lag := ateConfig.Lags().Add().SetName(lagName)
	lag.Protocol().Static().SetLagId(lagID)

	for i, p := range atePorts {
		port := ateConfig.Ports().Add().SetName(p.ID())
		mac, err := incrementMAC(lagAttrs.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lag.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(mac).SetName("LAGMember" + strconv.Itoa(i+1))
	}

	dev := ateConfig.Devices().Add().SetName(lagName + ".Dev")
	eth := dev.Ethernets().Add().SetName(lagAttrs.Name + "Eth-" + lagName).SetMac(lagAttrs.MAC)
	eth.Connection().SetLagName(lagName)

	ipv4 := eth.Ipv4Addresses().Add().SetName(lagAttrs.Name + ".IPv4")
	ipv4.SetAddress(lagAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(lagAttrs.IPv4Len))

	ipv6 := eth.Ipv6Addresses().Add().SetName(lagAttrs.Name + ".IPv6")
	ipv6.SetAddress(lagAttrs.IPv6).SetGateway(dutAttrs.IPv6).SetPrefix(uint32(lagAttrs.IPv6Len))

	bgp := dev.Bgp().SetRouterId(lagAttrs.IPv4)
	peerTypeV4 := gosnappi.BgpV4PeerAsType.IBGP
	peerTypeV6 := gosnappi.BgpV6PeerAsType.IBGP
	if isEBGP {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP
	}

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(lagAttrs.Name + ".BGPv4.Peer").SetPeerAddress(dutAttrs.IPv4).SetAsNumber(asn).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(lagAttrs.Name + ".BGPv6.Peer").SetPeerAddress(dutAttrs.IPv6).SetAsNumber(asn).SetAsType(peerTypeV6)

	if host3PrefixV4 != "" {
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag1.Name+".Host2.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
		addBGPRoutes(v6Peer.V6Routes().Add(), ateLag1.Name+".Host2.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ipv6.Address())
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag1.Name+".Host3.v4", host3PrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
	} else {
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag2.Name+".Host4.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
		addBGPRoutes(v6Peer.V6Routes().Add(), ateLag2.Name+".Host4.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ipv6.Address())
	}
	if isISIS {
		isis3LoopbackV4Net := []string{ate3Lo.IPv4 + "/" + strconv.Itoa(int(ate3Lo.IPv4Len))}
		isis3LoopbackV6Net := []string{ate3Lo.IPv6 + "/" + strconv.Itoa(int(ate3Lo.IPv6Len))}
		configureISIS(dev,
			ipv4.Address(),
			eth.Name(),
			append(isis3LoopbackV4Net, ateLag1.IPv4+"/"+strconv.Itoa(int(plenIPv4))), // IPv4 networks
			append(isis3LoopbackV6Net, ateLag1.IPv6+"/"+strconv.Itoa(int(plenIPv6))), // IPv6 networks
			ateSysID+"3",
		)
	}
}

// addBGPRoutes adds BGP route advertisements to an ATE device.
func addBGPRoutes[R any](routes R, name, startAddress string, prefixLen, count uint32, nextHop string) {
	switch r := any(routes).(type) {
	case gosnappi.BgpV4RouteRange:
		r.SetName(name).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL). // Use interface IP as next-hop
			SetNextHopIpv4Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	case gosnappi.BgpV6RouteRange:
		r.SetName(name).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL). // Use interface IP as next-hop
			SetNextHopIpv6Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	}
}

// configureISIS adds ISIS configuration to an ATE device.
func configureISIS(dev gosnappi.Device, routerID, ifName string, ipv4Nets, ipv6Nets []string, dutsysID string) {
	isis := dev.Isis().SetName(dev.Name() + ".ISIS").SetSystemId(dutsysID)
	isis.Basic().SetIpv4TeRouterId(routerID).SetHostname(dev.Name())

	// ISIS Interface Config
	isis.Interfaces().Add().
		SetEthName(ifName).
		SetName(dev.Name() + "IsisInt").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise IPv4 routes
	for i, net := range ipv4Nets {
		parts := strings.Split(net, "/")
		addr := parts[0]
		prefix, _ := strconv.Atoi(parts[1])
		isis.V4Routes().Add().
			SetName(fmt.Sprintf("%s.isis.v4net%d", dev.Name(), i)).
			Addresses().Add().
			SetAddress(addr).
			SetPrefix(uint32(prefix))
	}

	// Advertise IPv6 routes
	for i, net := range ipv6Nets {
		parts := strings.Split(net, "/")
		addr := parts[0]
		prefix, _ := strconv.Atoi(parts[1])
		isis.V6Routes().Add().
			SetName(fmt.Sprintf("%s.isis.v6net%d", dev.Name(), i)).
			Addresses().Add().
			SetAddress(addr).
			SetPrefix(uint32(prefix))
	}
}

// Validate all BGP neighbors are in ESTABLISHED state
func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice, neighbors []Neighbor) {
	t.Helper()
	t.Log("Verifying BGP neighbor sessions (IPv4 and IPv6)")

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for idx, nbr := range neighbors {
		t.Logf("Checking BGP IPv4 neighbor %s (Neighbor %d)", nbr.IPv4, idx+1)
		nbrPath := bgpPath.Neighbor(nbr.IPv4)

		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute,
			func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				currState, present := val.Val()
				return present && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP IPv4 state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatalf("BGP IPv4 session to neighbor %s not ESTABLISHED as expected", nbr.IPv4)
		}
		t.Logf("BGP IPv4 neighbor %s ESTABLISHED", nbr.IPv4)

		t.Logf("Checking BGP IPv6 neighbor %s (Neighbor %d)", nbr.IPv6, idx+1)
		nbrPathv6 := bgpPath.Neighbor(nbr.IPv6)

		_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute,
			func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				currState, present := val.Val()
				return present && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP IPv6 state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatalf("BGP IPv6 session to neighbor %s not ESTABLISHED as expected", nbr.IPv6)
		}
		t.Logf("BGP IPv6 neighbor %s ESTABLISHED", nbr.IPv6)
	}

	t.Log("All BGP IPv4 and IPv6 neighbors are ESTABLISHED.")
}

func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

func configureFlows(t *testing.T, otgConfig gosnappi.Config, macAddress string, dstPorts []string, incr int, immediateHeader bool) gosnappi.Flow {
	t.Helper()
	t.Logf("Adding Traffic Stream: %s", "Flow-"+strconv.Itoa(incr))
	flow := otgConfig.Flows().Add().SetName("Flow-" + strconv.Itoa(incr))
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(otgConfig.Ports().Items()[0].Name()).SetRxNames(dstPorts)
	flow.Size().SetFixed(trafficFrameSize)
	flow.Duration().FixedPackets().SetPackets(fixedPackets)
	flow.Rate().SetPercentage(ratePercent)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateP1.MAC)
	eth.Dst().SetValue(macAddress)

	ipOuter := flow.Packet().Add().Ipv4()
	ipOuter.Src().SetValue(ateP1.IPv4)
	if incr == 11 || incr == 12 {
		ipOuter.Dst().SetValue(ateP2.IPv4)
	} else if immediateHeader {
		ipOuter.Dst().SetValue(ateP2.IPv4)
	} else {
		ipOuter.Dst().SetValue(dcapIp)
	}
	udpOuter := flow.Packet().Add().Udp()
	if immediateHeader {
		udpOuter.SrcPort().SetValue(UdpSrcPort)
	} else {
		udpOuter.SrcPort().Increment().SetStart(UdpSrcPort).SetStep(1).SetCount(10)
	}
	if incr == 13 || incr == 14 {
		udpOuter.DstPort().SetValue(UdpDstPortNeg)
	} else {
		udpOuter.DstPort().SetValue(UdpDstPort)
	}

	// Flow-specific configuration from image table
	switch incr {
	case 1, 6:
		// Middle MPLS + IPv4 UDP for GUE
		mpls := flow.Packet().Add().Mpls()
		mpls.Label().SetValue(mplsLabel) // Example label

		ipMiddle := flow.Packet().Add().Ipv4()
		ipMiddle.Src().SetValue(ate1LoopbackIP)
		ipMiddle.Dst().SetValue(ateLag1.IPv4)

		udpMiddle := flow.Packet().Add().Udp()
		if immediateHeader {
			udpMiddle.SrcPort().SetValue(UdpSrcPort - 1)
		} else {
			udpMiddle.SrcPort().Increment().SetStart(UdpSrcPort - 1).SetStep(1).SetCount(10)
		}
		udpMiddle.DstPort().SetValue(UdpDstPort)

		if incr == 1 {
			ipInner := flow.Packet().Add().Ipv4()
			ipInner.Src().SetValue(host1IPv4Start)
			ipInner.Dst().SetValue(host3IPv4Start)
		} else {
			ipInner := flow.Packet().Add().Ipv6()
			ipInner.Src().SetValue(host1IPv6Start)
			ipInner.Dst().SetValue(host3IPv6Start)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	case 2, 4:
		ipInner := flow.Packet().Add().Ipv4()
		ipInner.Src().SetValue(host1IPv4Start)
		if incr == 2 {
			ipInner.Dst().SetValue(host2IPv4Start)
		} else {
			ipInner.Dst().SetValue(host4IPv4Start)
		}
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValue(testSrcPort)
		udp.DstPort().SetValue(testDstPort)
	case 3, 5:
		ipInner := flow.Packet().Add().Ipv4()
		ipInner.Src().SetValue(host1IPv4Start)
		if incr == 3 {
			ipInner.Dst().SetValue(host2IPv4Start)
		} else {
			ipInner.Dst().SetValue(host4IPv4Start)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	case 7, 9:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(host1IPv6Start)
		if incr == 7 {
			ipInner.Dst().SetValue(host2IPv6Start)
		} else {
			ipInner.Dst().SetValue(host4IPv6Start)
		}
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().Increment().SetStart(UdpSrcPort - 1).SetStep(1).SetCount(10)
		udp.DstPort().SetValue(UdpSrcPort - 2)
	case 8, 10:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(host1IPv6Start)
		if incr == 8 {
			ipInner.Dst().SetValue(host2IPv6Start)
		} else {
			ipInner.Dst().SetValue(host4IPv6Start)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	case 11, 12, 13, 14:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(host1IPv6Start)
		ipInner.Dst().SetValue(host4IPv6Start)
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	}
	return flow
}

func verifyFlowTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) bool {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	countersPath := gnmi.OTG().Flow(flowName).Counters()
	txRate := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())
	isWithinTolerance := func(v uint64) bool {
		return v >= txRate-tolerance && v <= txRate+tolerance
	}
	txVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.OutPkts().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Flow %q: TX did not reach expected count (%d)", flowName, txRate)
		return false
	}

	// Wait for RX to match TX exactly
	rxVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.InPkts().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Flow %q: RX packets did not match expected TX count (%d)", flowName, txRate)
		return false
	}

	txPkts, _ := txVal.Val()
	rxPkts, _ := rxVal.Val()
	t.Logf("Flow %q: TX=%d, RX=%d", flowName, txPkts, rxPkts)
	return true
}

// testLoadBalance to ensure 50:50 Load Balancing
func testLoadBalance(t *testing.T, ate *ondatra.ATEDevice, aggNames []string, flow gosnappi.Flow, aggregateAggName string) []uint64 {
	t.Helper()
	var rxs []uint64
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
	flowInFrames := flowMetrics.GetCounters().GetInPkts()
	for _, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		rxs = append(rxs, (metrics.GetCounters().GetInFrames()))
		inFrames := metrics.GetCounters().GetInFrames()
		if aggName == aggregateAggName {
			inFrames = inFrames - flowInFrames
		}
		rxs = append(rxs, inFrames)
	}
	var total uint64
	for _, rx := range rxs {
		total += rx
	}
	for idx, rx := range rxs {
		rxs[idx] = (rx * 100) / total
	}
	return rxs
}

func getPortRxPkts(t *testing.T, ate *ondatra.ATEDevice, flow gosnappi.Flow, rxPort string) {
	t.Helper()
	if rxPort != "" {
		// Constants for lower and upper bounds as percentage of total flow (e.g., 30% to 80%)
		const lowerPct = 30
		const upperPct = 81

		// Fetch flow-level InPkts
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
		flowInFrames := flowMetrics.GetCounters().GetInPkts()

		// Fetch port-level InFrames
		portMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(rxPort).State())
		portFrames := portMetrics.GetCounters().GetInFrames()

		// Calculate thresholds
		lowerBound := (flowInFrames * lowerPct) / 100
		upperBound := (flowInFrames * upperPct) / 100

		if portFrames >= lowerBound && portFrames <= upperBound {
			t.Logf("Port %s received %d packets within expected range [%d - %d] for flow %s: Load Balance Success",
				rxPort, portFrames, lowerBound, upperBound, flow.Name())
		} else {
			t.Errorf("Port %s received %d packets out of expected range [%d - %d] for flow %s: Load Balance Failed",
				rxPort, portFrames, lowerBound, upperBound, flow.Name())
		}
	}
}

func verifyTrafficFlowNegCase(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flow gosnappi.Flow) bool {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
	lostPkt := txPkts - rxPkts
	if got := (lostPkt * 100 / txPkts); got >= tolerance {
		return false
	}
	return true
}

func verifySinglePathTraffic(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
	portList := []string{
		otgConfig.Ports().Items()[1].Name(), // primary destination port
		otgConfig.Ports().Items()[2].Name(), // alternative path
		otgConfig.Ports().Items()[3].Name(),
	}
	aggNames := []string{
		otgConfig.Lags().Items()[0].Name(),
		otgConfig.Lags().Items()[1].Name(),
	}
	totalRx := uint64(0)
	nonZeroPorts := 0
	for _, port := range portList {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port).Counters().InFrames().State())
		t.Logf("Port %s received %d packets", port, rxPkts)
		if rxPkts > tolerance {
			nonZeroPorts++
		}
		totalRx += rxPkts
	}
	for _, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		inFrames := metrics.GetCounters().GetInFrames()
		t.Logf("Lag %s received %d packets", aggName, inFrames)
		if inFrames > tolerance {
			nonZeroPorts++
		}
	}
	if nonZeroPorts > tolerance {
		t.Fatalf("Expected traffic to follow a single path, but received on %d ports", nonZeroPorts)
	} else {
		t.Logf("PASS: All traffic followed a single path as expected")
	}
}

// Support method to execute GNMIC commands
func buildCliConfigRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
}
