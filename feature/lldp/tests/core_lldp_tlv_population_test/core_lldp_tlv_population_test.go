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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	portName = "port1"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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
		}, {
			desc:        "LLDP Disabled",
			lldpEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			dut, dutConf := configureNode(t, "dut1", test.lldpEnabled)
			ate, ateConf := configureNode(t, "dut2", true) // lldp is always enabled for the ATE
			dutPort := dut.Port(t, portName)
			atePort := ate.Port(t, portName)

			verifyNodeConfig(t, dut, dutPort, dutConf, test.lldpEnabled)
			verifyNodeConfig(t, ate, atePort, ateConf, true)
			if test.lldpEnabled {
				verifyNodeTelemetry(t, dut, ate, dutPort, atePort, test.lldpEnabled)
				verifyNodeTelemetry(t, ate, dut, atePort, dutPort, test.lldpEnabled)
			} else {
				verifyNodeTelemetry(t, dut, ate, dutPort, atePort, test.lldpEnabled)
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

// verifyNodeTelemetry verifies the telemetry values from the node such as port LLDP neighbor info.
func verifyNodeTelemetry(t *testing.T, node, peer gnmi.DeviceOrOpts, nodePort, peerPort *ondatra.Port, lldpEnabled bool) {
	interfacePath := gnmi.OC().Lldp().Interface(nodePort.Name())

	// LLDP Disabled
	lldpTelemetry := gnmi.Get(t, node, gnmi.OC().Lldp().Enabled().State())
	if lldpEnabled != lldpTelemetry {
		t.Errorf("LLDP enabled telemetry got: %t, want: %t.", lldpTelemetry, lldpEnabled)
	}

	// Ensure that DUT does not generate any LLDP messages irrespective of the
	// configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
	// on any interface.
	var gotLen int
	if !lldpEnabled {
		if _, ok := gnmi.Watch(t, node, interfacePath.State(), time.Minute, func(val *ygnmi.Value[*oc.Lldp_Interface]) bool {
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
	peerState := gnmi.Get(t, peer, gnmi.OC().Lldp().State())

	// Get the LLDP port neighbor ID and state of the node.
	if _, ok := gnmi.Watch(t, node, interfacePath.State(), time.Minute, func(val *ygnmi.Value[*oc.Lldp_Interface]) bool {
		intf, present := val.Val()
		return present && len(intf.Neighbor) > 0
	}).Await(t); !ok {
		t.Error("Number of neighbors: got 0, want > 0.")
		return
	}

	nbrInterfaceID := gnmi.GetAll(t, node, interfacePath.NeighborAny().Id().State())[0]
	nbrStatePath := interfacePath.Neighbor(nbrInterfaceID) // *telemetry.Lldp_Interface_NeighborPath
	gotNbrState := gnmi.Get(t, node, nbrStatePath.State()) // *telemetry.Lldp_Interface_Neighbor which is a ValidatedGoStruct.
	fptest.LogQuery(t, "Node port neighbor", nbrStatePath.State(), gotNbrState)

	// Verify the neighbor state against expected values.
	wantNbrState := &oc.Lldp_Interface_Neighbor{
		ChassisId:     peerState.ChassisId,
		ChassisIdType: peerState.ChassisIdType,
		PortId:        ygot.String(peerPort.Name()),
		PortIdType:    oc.Lldp_PortIdType_INTERFACE_NAME,
		SystemName:    peerState.SystemName,
	}
	confirm.State(t, wantNbrState, gotNbrState)
}
