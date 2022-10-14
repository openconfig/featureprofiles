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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.

package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

// Some of the variables defined below like DutAS, AteAS, PeerGrpName
// RouteCount and IsisInstance are used in other files which import
// setup.go

const (
	DutAS                 = 64500
	AteAS                 = 64501
	ateAS2                = 64502
	PeerGrpName           = "BGP-PEER-GROUP"
	plenIPv4              = 30
	dutStartIPAddr        = "192.0.2.1"
	ateStartIPAddr        = "192.0.2.2"
	RouteCount            = 200
	advertiseBGPRoutesv4  = "203.0.113.1"
	authPassword          = "ISISAuthPassword"
	advertiseISISRoutesv4 = "198.18.0.0"
	IsisInstance          = "DEFAULT"
)

var (
	dutIPPool = make(map[string]net.IP)
	ateIPPool = make(map[string]net.IP)
)

// BuildIPPool is to Build pool of ip addresses for both DUT and ATE interfaces.
// It reads ports given in binding file to calculate ip addresses needed.
func BuildIPPool(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var dutIPIndex, ipSubnet, ateIPIndex int = 1, 2, 2
	var endSubnetIndex = 253
	for _, dp := range dut.Ports() {
		dutNextIP := nextIP(net.ParseIP(dutStartIPAddr), dutIPIndex, ipSubnet)
		ateNextIP := nextIP(net.ParseIP(ateStartIPAddr), ateIPIndex, ipSubnet)
		dutIPPool[dp.ID()] = dutNextIP
		ateIPPool[dp.ID()] = ateNextIP

		// Increment dut and ate host ip index by 4
		dutIPIndex = dutIPIndex + 4
		ateIPIndex = ateIPIndex + 4

		// Reset dut and ate IP indexes when it is greater endSubnetIndex
		if dutIPIndex > int(endSubnetIndex) {
			ipSubnet = ipSubnet + 1
			dutIPIndex = 1
			ateIPIndex = 2
		}
	}

}

// nextIP returns ip address based on hostindex and subnetindex provided.
func nextIP(ip net.IP, hostIndex int, subnetIndex int) net.IP {
	s := ip.String()
	sa := strings.Split(s, ".")
	sa[2] = strconv.Itoa(subnetIndex)
	sa[3] = strconv.Itoa(hostIndex)
	s = strings.Join(sa, ".")
	return net.ParseIP(s)
}

// CreateGNMIUpdate is to create GNMI update message . It will marshal the input
// strings provided and return gpb update message to the calling function
func CreateGNMIUpdate(map1 string, map2 string, configElem []M) *gpb.Update {

	j := map[string]interface{}{
		map1: map[string]interface{}{
			map2: configElem,
		},
	}

	v, err := json.Marshal(j)
	if err != nil {
		err1 := fmt.Errorf("marshal of intf config failed with unexpected error: %v", err)
		fmt.Println(err1.Error())
	}
	update := &gpb.Update{
		Path: &gpb.Path{Elem: []*gpb.PathElem{}},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: v}},
	}
	return update
}

// BuildOCInterfaceUpdate function is to build  OC config for interfaces.
// It reads ports from binding file and returns gpb update message
// which will have configurations for all the ports.
func BuildOCInterfaceUpdate(t *testing.T) *gpb.Update {
	dut := ondatra.DUT(t, "dut")
	type M map[string]interface{}
	var intfConfig []M

	for _, dp := range dut.Ports() {
		elem := map[string]interface{}{
			"name": dp.Name(),
			"config": map[string]interface{}{
				"enabled":     true,
				"description": "from oc",
				"name":        dp.Name(),
				"type":        "iana-if-type:ethernetCsmacd",
			},
			"subinterfaces": map[string]interface{}{
				"subinterface": []M{
					{
						"config": map[string]interface{}{
							"index": 0,
						},
						"index": 0,
						"openconfig-if-ip:ipv4": map[string]interface{}{
							"addresses": map[string]interface{}{
								"address": []M{
									{
										"ip": dutIPPool[dp.ID()],
										"config": map[string]interface{}{
											"ip":            dutIPPool[dp.ID()],
											"prefix-length": plenIPv4,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		intfConfig = append(intfConfig, elem)
	}

	update := CreateGNMIUpdate("interfaces", "interface", intfConfig)
	return update
}

// ConfigureATE function is to configure ate ports with ipv4 , bgp
// and isis peers.
func ConfigureATE(t *testing.T, ate *ondatra.ATEDevice) {
	topo := ate.Topology().New()

	for _, dp := range ate.Ports() {
		atePortAttr := attrs.Attributes{
			Name:    "ate" + dp.ID(),
			IPv4:    ateIPPool[dp.ID()].String(),
			IPv4Len: plenIPv4,
		}
		iDut1 := topo.AddInterface(dp.Name()).WithPort(dp)
		iDut1.IPv4().WithAddress(atePortAttr.IPv4CIDR()).WithDefaultGateway(dutIPPool[dp.ID()].String())

		// Add BGP routes and ISIS routes , ate port1 is ingress port.
		if dp.ID() == "port1" {
			//port1 is ingress port.
			// Add BGP on ATE
			bgpDut1 := iDut1.BGP()
			bgpDut1.AddPeer().WithPeerAddress(dutIPPool[dp.ID()].String()).WithLocalASN(ateAS2).
				WithTypeExternal()

			// Add BGP on ATE
			isisDut1 := iDut1.ISIS()
			isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(dutIPPool[dp.ID()].String()).WithAuthMD5(authPassword)

			netCIDR := fmt.Sprintf("%s/%d", advertiseBGPRoutesv4, 32)
			bgpNeti1 := iDut1.AddNetwork("bgpNeti1")
			bgpNeti1.IPv4().WithAddress(netCIDR).WithCount(routeCount)
			bgpNeti1.BGP().WithNextHopAddress(atePortAttr.IPv4)

			netCIDR = fmt.Sprintf("%s/%d", advertiseISISRoutesv4, 32)
			isisnet1 := iDut1.AddNetwork("isisnet1")
			isisnet1.IPv4().WithAddress(netCIDR).WithCount(routeCount)
			isisnet1.ISIS().WithActive(true).WithIPReachabilityMetric(20)

			continue
		}

		// Add BGP on ATE
		bgpDut1 := iDut1.BGP()
		bgpDut1.AddPeer().WithPeerAddress(dutIPPool[dp.ID()].String()).WithLocalASN(ateAS).
			WithTypeExternal()

		// Add BGP on ATE
		isisDut1 := iDut1.ISIS()
		isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(dutIPPool[dp.ID()].String()).WithAuthMD5(authPassword)

	}

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
}

// CreateGNMISetRequest function is to create GNMI setRequest message
// and returns gnmi set request to the calling function.
func CreateGNMISetRequest(j map[string]interface{}) *gpb.SetRequest {
	v, err := json.Marshal(j)
	if err != nil {
		err1 := fmt.Errorf("marshal of intf config failed with unexpected error: %v", err)
		fmt.Println(err1.Error())
	}

	update := &gpb.Update{
		Path: &gpb.Path{Elem: []*gpb.PathElem{}},
		Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: v}},
	}

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			update,
		},
	}
	return gpbSetRequest
}

// ConfigureGNMISetRequest function to used to configure GNMI setRequest on DUT.
func ConfigureGNMISetRequest(t *testing.T, gpbSetRequest *gpb.SetRequest) {
	dut := ondatra.DUT(t, "dut")

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	response, err := gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
	t.Log("gnmiClient Set Response for OC modelled config")
	t.Log(response)
}

// BuildOCBGPUpdate function to used build OC config for configuring
// bgp on DUT , one peer for one physical interface will be configured.
func BuildOCBGPUpdate(t *testing.T) *gpb.Update {
	dut := ondatra.DUT(t, "dut")
	type M map[string]interface{}
	var bgpNbrConfig []M
	for _, dp := range dut.Ports() {
		asNum := ateAS
		if dp.ID() == "port1" {
			asNum = ateAS2
		}
		elem := map[string]interface{}{
			"neighbor-address": ateIPPool[dp.ID()],
			"config": map[string]interface{}{
				"peer-group":       peerGrpName,
				"neighbor-address": ateIPPool[dp.ID()],
				"enabled":          true,
				"peer-as":          asNum,
			},
		}
		bgpNbrConfig = append(bgpNbrConfig, elem)
	}

	niConfig := []M{
		{
			"name": *deviations.DefaultNetworkInstance,
			"config": map[string]interface{}{
				"type":    "DEFAULT_INSTANCE",
				"enabled": true,
			},
			"protocols": map[string]interface{}{
				"protocol": []M{
					{
						"identifier": "BGP",
						"name":       "BGP",
						"bgp": map[string]interface{}{
							"global": map[string]interface{}{
								"config": map[string]interface{}{
									"as":        dutAS,
									"router-id": dutStartIPAddr,
								},
								"afi-safis": map[string]interface{}{
									"afi-safi": []M{
										{
											"afi-safi-name": "IPV4_UNICAST",
											"config": map[string]interface{}{
												"afi-safi-name": "IPV4_UNICAST",
												"enabled":       true,
											},
										},
									},
								},
							},
							"peer-groups": map[string]interface{}{
								"peer-group": []M{
									{
										"peer-group-name": peerGrpName,
										"config": map[string]interface{}{
											"peer-group-name": peerGrpName,
											"peer-as":         ateAS,
										},
									},
								},
							},
							"neighbors": map[string]interface{}{
								"neighbor": bgpNbrConfig,
							},
						},
					},
				},
			},
		},
	}

	update := CreateGNMIUpdate("network-instances", "network-instance", niConfig)
	return update
}

// BuildOCISISUpdate function to used build OC ISIS configs
// on DUT , one isis peer per port is configured.
func BuildOCISISUpdate(t *testing.T) *gpb.Update {
	dut := ondatra.DUT(t, "dut")
	type M map[string]interface{}
	var isisIntfConfig []M
	for _, dp := range dut.Ports() {
		elem1 := map[string]interface{}{
			"interface-id": dp.Name(),
			"config": map[string]interface{}{
				"enabled":       true,
				"hello-padding": "ADAPTIVE",
				"circuit-type":  "POINT_TO_POINT",
			},
			"authentication": map[string]interface{}{
				"config": map[string]interface{}{
					"enabled":       true,
					"auth-password": authPassword,
					"auth-mode":     "MD5",
					"auth-type":     "openconfig-keychain-types:SIMPLE_KEY",
				},
			},
			"levels": map[string]interface{}{
				"level": []M{
					{
						"level-number": 2,
						"timers": map[string]interface{}{
							"config": map[string]interface{}{
								"hello-interval":   1,
								"hello-multiplier": 5,
							},
						},
						"afi-safi": map[string]interface{}{
							"af": []M{
								{
									"afi-name":  "IPV4",
									"safi-name": "UNICAST",
									"config": map[string]interface{}{
										"afi-name":  "IPV4",
										"safi-name": "UNICAST",
										"metric":    200,
										"enabled":   true,
									},
								},
							},
						},
					},
				},
			},
		}
		isisIntfConfig = append(isisIntfConfig, elem1)
	}

	niConfig := []M{
		{
			"name": *deviations.DefaultNetworkInstance,
			"config": map[string]interface{}{
				"type":      "DEFAULT_INSTANCE",
				"router-id": dutStartIPAddr,
				"enabled":   true,
			},
			"protocols": map[string]interface{}{
				"protocol": []M{
					{
						"identifier": "ISIS",
						"name":       isisInstance,
						"isis": map[string]interface{}{
							"global": map[string]interface{}{
								"config": map[string]interface{}{
									"authentication-check": true,
								},
								"lsp-bit": map[string]interface{}{
									"overload-bit": map[string]interface{}{
										"config": map[string]interface{}{
											"set-bit": false,
										},
									},
								},
								"timers": map[string]interface{}{
									"config": map[string]interface{}{
										"lsp-lifetime-interval": 600,
									},
									"spf": map[string]interface{}{
										"config": map[string]interface{}{
											"spf-hold-interval":  5000,
											"spf-first-interval": 600,
										},
									},
								},
							},
							"levels": map[string]interface{}{
								"level": []M{
									{
										"level-number": 1,
										"config": map[string]interface{}{
											"enabled": false,
										},
									},
									{
										"level-number": 2,
										"config": map[string]interface{}{
											"enabled":      true,
											"metric-style": "WIDE_METRIC",
										},
										"authentication": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":       true,
												"auth-password": authPassword,
												"auth-mode":     "MD5",
												"auth-type":     "openconfig-keychain-types:SIMPLE_KEY",
											},
										},
									},
								},
							},
							"interfaces": map[string]interface{}{
								"interface": isisIntfConfig,
							},
						},
					},
				},
			},
		},
	}

	update := CreateGNMIUpdate("network-instances", "network-instance", niConfig)
	return update
}

// VerifyISISTelemetry function to used verify ISIS telemetry on DUT
// using OC isis telemetry path.
func VerifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "DEFAULT").Isis()
	for _, dp := range dut.Ports() {
		nbrPath := statePath.Interface(dp.Name())
		_, ok := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().Watch(t, time.Minute,
			func(val *telemetry.QualifiedE_IsisTypes_IsisInterfaceAdjState) bool {
				return val.IsPresent() && val.Val(t) == telemetry.IsisTypes_IsisInterfaceAdjState_UP
			}).Await(t)
		if !ok {
			fptest.LogYgot(t, fmt.Sprintf("IS-IS state on %v has no adjacencies", dp.Name()), nbrPath, nbrPath.Get(t))
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// VerifyBgpTelemetry function to verify BGP telemetry on DUT using
// BGP OC telemetry path.
func VerifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, peerAddr := range ateIPPool {
		nbrIP := peerAddr.String()
		nbrPath := statePath.Neighbor(nbrIP)

		// Get BGP adjacency state
		_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
			return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
			t.Fatal("No BGP neighbor formed")
		}
		status := nbrPath.SessionState().Get(t)
		if want := telemetry.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbrIP, status, want)
		}
	}
}
