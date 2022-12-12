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

package core_lldp_tlv_population_test

import (
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

type lldpTestParameters struct {
	SystemName string
	MACAddress string
	OtgName    string
}

type lldpNeighbors struct {
	systemName    string
	portId        string
	portIdType    otgtelemetry.E_LldpNeighbor_PortIdType
	chassisId     string
	chassisIdType otgtelemetry.E_LldpNeighbor_ChassisIdType
}

const (
	portName = "port1"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	// Ethernet configuration.
	ateSrc = attrs.Attributes{
		Name: "ateSrc",
		MAC:  "02:00:01:01:01:01",
	}

	// LLDP configuration.
	lldpSrc = lldpTestParameters{
		SystemName: "ixia-otg",
		MACAddress: "aa:bb:00:00:00:00",
		OtgName:    "ixia-otg",
	}
)

// Determine LLDP advertisement and reception operates correctly.
// Since ATE(Ixia) does not implement LLDP API, we are using
// DUT-DUT setup for topology.
//
// Topology:
//
//	dut1:port1 <--> dut2:port1
func TestCoreLLDPTLVPopulation(t *testing.T) {
	tests := []struct {
		desc        string
		lldpEnabled bool
	}{
		{
			desc:        "LLDP Enabled",
			lldpEnabled: true,
		},
		{
			desc:        "LLDP Disabled",
			lldpEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {

			// DUT configuration.
			t.Log("Configure DUT.")
			dut, dutConf := configureNode(t, "dut", test.lldpEnabled)
			dutPort := dut.Port(t, portName)
			verifyNodeConfig(t, dut, dutPort, dutConf, test.lldpEnabled)

			// ATE Configuration.
			t.Log("Configure ATE")
			ate := ondatra.ATE(t, "ate")
			otg := ate.OTG()
			otgConfig := configureATE(t, otg)
			t.Log(otgConfig.ToJson())

			waitFor(func() bool { return checkOtgLldpMetrics(t, otg, otgConfig, test.lldpEnabled) }, t)

			dutPeerState := lldpNeighbors{
				systemName:    lldpSrc.SystemName,
				chassisId:     lldpSrc.MACAddress,
				chassisIdType: otgtelemetry.LldpNeighbor_ChassisIdType_MAC_ADDRESS,
				portId:        ate.Port(t, portName).Name(),
				portIdType:    otgtelemetry.LldpNeighbor_PortIdType_INTERFACE_NAME,
			}

			if test.lldpEnabled {
				expOtgLldpNeighbors := map[string][]lldpNeighbors{
					"ixia-otg": {
						{
							systemName:    dutConf.GetSystemName(),
							portId:        dutPort.Name(),
							portIdType:    otgtelemetry.LldpNeighbor_PortIdType_INTERFACE_NAME,
							chassisId:     strings.ToUpper(dutConf.GetChassisId()),
							chassisIdType: otgtelemetry.E_LldpNeighbor_ChassisIdType(dutConf.GetChassisIdType()),
						},
					},
				}

				waitFor(func() bool { return checkOtgLldpNeighbors(t, otg, otgConfig, expOtgLldpNeighbors) }, t)
				verifyDUTTelemetry(t, dut, dutPort, dutConf, dutPeerState, test.lldpEnabled)
			} else {
				expOtgLldpNeighbors := map[string][]lldpNeighbors{
					"ixia-otg": {},
				}

				waitFor(func() bool { return checkOtgLldpNeighbors(t, otg, otgConfig, expOtgLldpNeighbors) }, t)
			}
		})
	}
}

// configureNode configures LLDP on a single node.
func configureNode(t *testing.T, name string, lldpEnabled bool) (*ondatra.DUTDevice, *oc.Lldp) {
	node := ondatra.DUT(t, name)
	p := node.Port(t, portName)
	lldp := gnmi.OC().Lldp()

	gnmi.Replace(t, node, lldp.Enabled().Config(), lldpEnabled)

	if lldpEnabled {
		gnmi.Replace(t, node, lldp.Interface(p.Name()).Enabled().Config(), lldpEnabled)
	}

	return node, gnmi.GetConfig(t, node, lldp.Config())
}

func configureATE(t *testing.T, otg *otg.OTG) gosnappi.Config {

	// Device configuration + Ethernet configuration.
	config := otg.NewConfig(t)
	srcPort := config.Ports().Add().SetName(portName)
	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	srcEth.SetPortName(srcPort.Name()).SetMac(ateSrc.MAC)

	// LLDP configuration.
	lldp := config.Lldp().Add()
	lldp.SystemName().SetValue(lldpSrc.SystemName)
	lldp.SetName(lldpSrc.OtgName)
	lldp.Connection().SetPortName(portName)
	lldp.ChassisId().MacAddressSubtype().SetValue(lldpSrc.MACAddress)

	// Push config and start protocol.
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

// verifyNodeConfig verifies the config by comparing against the telemetry state object.
func verifyNodeConfig(t *testing.T, node gnmi.DeviceOrOpts, port *ondatra.Port, conf *oc.Lldp, lldpEnabled bool) {
	statePath := gnmi.OC().Lldp()
	state := gnmi.Get(t, node, statePath.State())
	fptest.LogQuery(t, "Node LLDP", statePath.State(), state)

	if lldpEnabled != state.GetEnabled() {
		t.Errorf("LLDP enabled got: %t, want: %t.", state.GetEnabled(), lldpEnabled)
	}
	if state.GetChassisId() != "" {
		t.Logf("LLDP ChassisId got: %s", state.GetChassisId())
	} else {
		t.Errorf("LLDP chassisID is not proper, got %s", state.GetChassisId())
	}
	if state.GetChassisIdType() != 0 {
		t.Logf("LLDP ChassisIdType got: %s", state.GetChassisIdType())
	} else {
		t.Errorf("LLDP chassisIdType is not proper, got %s", state.GetChassisIdType())
	}
	if state.GetSystemName() != "" {
		t.Logf("LLDP SystemName got: %s", state.GetSystemName())
	} else {
		t.Errorf("LLDP SystemName is not proper, got %s", state.GetSystemName())
	}

	got := state.GetInterface(port.Name()).GetName()
	want := conf.GetInterface(port.Name()).GetName()

	if lldpEnabled && got != want {
		t.Errorf("LLDP interfaces/interface/state/name = %s, want %s", got, want)
	}
}

func waitFor(fn func() bool, t testing.TB) {
	start := time.Now()
	for {
		done := fn()
		if done {
			t.Logf("Expected LLDP Stats/Metric received")
			break
		}
		if time.Since(start) > 65*time.Second {
			t.Error("Timeout while waiting for expected stats...")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// checkOtgLldpMetrics verifies OTG side lldp Metrics values based on DUT side lldp is enabled or not
func checkOtgLldpMetrics(t *testing.T, otg *otg.OTG, c gosnappi.Config, lldpEnabled bool) bool {
	otgutils.LogLLDPMetrics(t, otg, c)
	for _, lldp := range c.Lldp().Items() {
		lldpName := lldp.Name()
		lldpState := gnmi.Get(t, otg, gnmi.OTG().LldpInterface(lldpName).State())
		lldpStateCounters := lldpState.GetCounters()

		if lldpStateCounters.GetFrameOut() == 0 {
			return false
		}

		if lldpEnabled {
			if lldpStateCounters.GetFrameIn() == 0 {
				return false
			}
		} else {
			if lldpStateCounters.GetFrameIn() != 0 {
				return false
			}
		}
	}
	return true
}

// checkOtgLldpMetrics verifies OTG side lldp neighbor states
func checkOtgLldpNeighbors(t *testing.T, otg *otg.OTG, c gosnappi.Config, expLldpNeighbors map[string][]lldpNeighbors) bool {
	// otgutils.LogLLDPNeighborStates(t, otg, c)

	for lldp, lldpNeighbors := range expLldpNeighbors {
		lldpState := gnmi.Get(t, otg, gnmi.OTG().LldpInterface(lldp).State())

		if len(lldpNeighbors) == 0 {
			if lldpState.LldpNeighborDatabase != nil {
				return false
			}
		} else {
			neighbors := lldpState.GetLldpNeighborDatabase().LldpNeighbor
			for _, lldpNeighbor := range lldpNeighbors {
				neighborFound := false
				for _, neighbor := range neighbors {
					if neighbor.GetChassisId() == lldpNeighbor.chassisId {
						if neighbor.GetChassisIdType() == lldpNeighbor.chassisIdType {
							if neighbor.GetPortId() == lldpNeighbor.portId {
								if neighbor.GetPortIdType() == lldpNeighbor.portIdType {
									if neighbor.GetSystemName() == lldpNeighbor.systemName {
										neighborFound = true
										break
									}
								}
							}
						}
					}
				}

				if !neighborFound {
					return false
				}
			}
		}

	}
	return true
}

// verifyNodeTelemetry verifies the telemetry values from the node such as port LLDP neighbor info.
func verifyDUTTelemetry(t *testing.T, dut *ondatra.DUTDevice, nodePort *ondatra.Port, conf *oc.Lldp, dutPeerState lldpNeighbors, lldpEnabled bool) {
	verifyNodeConfig(t, dut, nodePort, conf, lldpEnabled)
	interfacePath := gnmi.OC().Lldp().Interface(nodePort.Name())

	// LLDP Disabled
	// lldpTelemetry := gnmi.Get(t, dut, gnmi.OC().Lldp().Enabled().State())
	// if lldpEnabled != lldpTelemetry {
	// 	t.Errorf("LLDP enabled telemetry got: %t, want: %t.", lldpTelemetry, lldpEnabled)
	// }

	// Ensure that DUT does not generate any LLDP messages irrespective of the
	// configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
	// on any interface.
	var gotLen int
	if !lldpEnabled {
		if _, ok := gnmi.Watch(t, dut, interfacePath.State(), time.Minute, func(val *ygnmi.Value[*oc.Lldp_Interface]) bool {
			intf, present := val.Val()
			if !present {
				return false
			}
			gotLen = len(intf.Neighbor)
			return gotLen == 0
		}).Await(t); !ok {
			t.Errorf("Number of neighbors got: %d, want: 0.", gotLen)
		}
		return
	}

	// LLDP Enabled
	// Get the LLDP state of the peer.

	// Get the LLDP port neighbor ID and state of the node.
	if _, ok := gnmi.Watch(t, dut, interfacePath.State(), time.Minute, func(val *ygnmi.Value[*oc.Lldp_Interface]) bool {
		intf, present := val.Val()
		return present && len(intf.Neighbor) > 0
	}).Await(t); !ok {
		t.Error("Number of neighbors: got 0, want > 0.")
		return
	}

	nbrInterfaceID := gnmi.GetAll(t, dut, interfacePath.NeighborAny().Id().State())[0]
	nbrStatePath := interfacePath.Neighbor(nbrInterfaceID) // *telemetry.Lldp_Interface_NeighborPath
	gotNbrState := gnmi.Get(t, dut, nbrStatePath.State())  // *telemetry.Lldp_Interface_Neighbor which is a ValidatedGoStruct.
	fptest.LogQuery(t, "Node port neighbor", nbrStatePath.State(), gotNbrState)

	t.Log(dutPeerState)

	// Verify the neighbor state against expected values.
	wantNbrState := &oc.Lldp_Interface_Neighbor{
		ChassisId:     &dutPeerState.chassisId,
		ChassisIdType: oc.E_Lldp_ChassisIdType(dutPeerState.chassisIdType),
		PortId:        &dutPeerState.portId,
		PortIdType:    oc.E_Lldp_PortIdType(dutPeerState.portIdType),
		SystemName:    &dutPeerState.systemName,
	}
	confirm.State(t, wantNbrState, gotNbrState)
}
