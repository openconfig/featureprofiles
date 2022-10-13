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
	"github.com/openconfig/ondatra/telemetry/device"
	"github.com/openconfig/ygot/ygot"

	telemetry "github.com/openconfig/ondatra/telemetry"
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

			verifyNodeConfig(t, dut.Telemetry(), dutPort, dutConf, test.lldpEnabled)
			verifyNodeConfig(t, ate.Telemetry(), atePort, ateConf, true)
			if test.lldpEnabled {
				verifyNodeTelemetry(t, dut.Telemetry(), ate.Telemetry(), dutPort, atePort, test.lldpEnabled)
				verifyNodeTelemetry(t, ate.Telemetry(), dut.Telemetry(), atePort, dutPort, test.lldpEnabled)
			} else {
				verifyNodeTelemetry(t, dut.Telemetry(), ate.Telemetry(), dutPort, atePort, test.lldpEnabled)
			}
		})
	}
}

// configureNode configures LLDP on a single node.
func configureNode(t *testing.T, name string, lldpEnabled bool) (*ondatra.DUTDevice, *telemetry.Lldp) {
	node := ondatra.DUT(t, name)
	p := node.Port(t, portName)
	lldp := node.Config().Lldp()

	lldp.Enabled().Replace(t, lldpEnabled)

	if lldpEnabled {
		lldp.Interface(p.Name()).Enabled().Replace(t, lldpEnabled)
	}

	return node, lldp.Get(t)
}

// verifyNodeConfig verifies the config by comparing against the telemetry state object.
func verifyNodeConfig(t *testing.T, nodeTelemetry *device.DevicePath, port *ondatra.Port, conf *telemetry.Lldp, lldpEnabled bool) {
	statePath := nodeTelemetry.Lldp()
	state := statePath.Get(t)
	fptest.LogYgot(t, "Node LLDP", statePath, state)

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
	if conf.GetInterface(port.Name()).GetName() != state.GetInterface(port.Name()).GetName() {
		t.Errorf("LLDP interfaces/interface/state/name got: %s, want: %s.", state.GetInterface(port.Name()).GetName(),
			conf.GetInterface(port.Name()).GetName())
	}
}

// verifyNodeTelemetry verifies the telemetry values from the node such as port LLDP neighbor info.
func verifyNodeTelemetry(t *testing.T, nodeTelemetry, peerTelemetry *device.DevicePath, nodePort, peerPort *ondatra.Port, lldpEnabled bool) {
	interfacePath := nodeTelemetry.Lldp().Interface(nodePort.Name())

	// LLDP Disabled
	lldpTelemetry := nodeTelemetry.Lldp().Enabled().Get(t)
	if lldpEnabled != lldpTelemetry {
		t.Errorf("LLDP enabled telemetry got: %t, want: %t.", lldpTelemetry, lldpEnabled)
	}

	// Ensure that DUT does not generate any LLDP messages irrespective of the
	// configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
	// on any interface.
	if !lldpEnabled {
		if got, ok := interfacePath.Watch(t, time.Minute, func(val *telemetry.QualifiedLldp_Interface) bool {
			return val.IsPresent() && len(val.Val(t).Neighbor) == 0
		}).Await(t); !ok {
			t.Errorf("Number of neighbors got: %d, want: 0.", len(got.Val(t).Neighbor))
		}
		return
	}

	// LLDP Enabled
	// Get the LLDP state of the peer.
	peerState := peerTelemetry.Lldp().Get(t)

	// Get the LLDP port neighbor ID and state of the node.
	if _, ok := interfacePath.Watch(t, time.Minute, func(val *telemetry.QualifiedLldp_Interface) bool {
		return val.IsPresent() && len(val.Val(t).Neighbor) > 0
	}).Await(t); !ok {
		t.Error("Number of neighbors: got 0, want > 0.")
		return
	}

	nbrInterfaceID := interfacePath.NeighborAny().Id().Get(t)[0]
	nbrStatePath := interfacePath.Neighbor(nbrInterfaceID) // *telemetry.Lldp_Interface_NeighborPath
	gotNbrState := nbrStatePath.Get(t)                     // *telemetry.Lldp_Interface_Neighbor which is a ValidatedGoStruct.
	fptest.LogYgot(t, "Node port neighbor", nbrStatePath, gotNbrState)

	// Verify the neighbor state against expected values.
	wantNbrState := &telemetry.Lldp_Interface_Neighbor{
		ChassisId:     peerState.ChassisId,
		ChassisIdType: peerState.ChassisIdType,
		PortId:        ygot.String(peerPort.Name()),
		PortIdType:    telemetry.LldpTypes_PortIdType_INTERFACE_NAME,
		SystemName:    peerState.SystemName,
	}
	confirm.State(t, wantNbrState, gotNbrState)
}
