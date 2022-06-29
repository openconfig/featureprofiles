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

package lldp_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry/device"
	"github.com/openconfig/ygot/ygot"

	telemetry "github.com/openconfig/ondatra/telemetry"
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
		lldpEnabled bool
	}{
		{
			lldpEnabled: true,
		}, {
			lldpEnabled: false,
		},
	}

	for _, test := range tests {
		dut, dutConf := configureNode(t, "dut1", test.lldpEnabled)
		ate, ateConf := configureNode(t, "dut2", true) //lldp is always enabled for the ATE
		dutPort := dut.Port(t, "port1")
		atePort := ate.Port(t, "port1")

		verifyNode(t, dut.Telemetry(), ate.Telemetry(), dutPort, atePort, dutConf)
		verifyNode(t, ate.Telemetry(), dut.Telemetry(), atePort, dutPort, ateConf)
	}
}

// configureNode configures LLDP on a single node.
func configureNode(t *testing.T, name string, lldpEnabled bool) (*ondatra.DUTDevice, *telemetry.Lldp) {
	node := ondatra.DUT(t, name)
	p := node.Port(t, "port1")
	lldp := node.Config().Lldp()

	lldp.Enabled().Replace(t, lldpEnabled)

	if lldpEnabled {
		lldp.Interface(p.Name()).Enabled().Replace(t, lldpEnabled)
	}

	return node, lldp.Get(t)
}

// verifyNode verifies the telemetry from the node for LLDP functionality.
func verifyNode(t *testing.T, nodeTelemetry, peerTelemetry *device.DevicePath, nodePort, peerPort *ondatra.Port, conf *telemetry.Lldp) {
	verifyNodeConfig(t, nodeTelemetry, conf)
	verifyNodeTelemetry(t, nodeTelemetry, peerTelemetry, nodePort, peerPort)
}

// verifyNodeConfig verifies the config by comparing against the telemetry state object.
func verifyNodeConfig(t *testing.T, nodeTelemetry *device.DevicePath, conf *telemetry.Lldp) {
	statePath := nodeTelemetry.Lldp()
	state := statePath.Get(t)
	fptest.LogYgot(t, "Node LLDP", statePath, state)

	if state != nil && state.Enabled == nil {
		state.Enabled = ygot.Bool(true)
	}

	confirm.State(t, conf, state)
}

// verifyNodeTelemetry verifies the telemetry values from the node such as port LLDP neighbor info.
func verifyNodeTelemetry(t *testing.T, nodeTelemetry, peerTelemetry *device.DevicePath, nodePort, peerPort *ondatra.Port) {

	// Ensure that DUT does not generate any LLDP messages irrespective of the
	// configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
	// on any interface.
	if !nodeTelemetry.Lldp().Enabled().Get(t) {
		interfaces := nodeTelemetry.Lldp().Interface(nodePort.Name()).NeighborAny().Id().Get(t)
		if len(interfaces) > 0 {
			t.Errorf("Number of neighbors: got %d, want zero.", len(interfaces))
		}
		return
	}
	// Get the LLDP state of the peer.
	peerState := peerTelemetry.Lldp().Get(t)

	// Get the LLDP port neighbor ID and state of the node.
	nbrInterfaceID := nodeTelemetry.Lldp().Interface(nodePort.Name()).NeighborAny().Id().Get(t)[0]
	nbrStatePath := nodeTelemetry.Lldp().Interface(nodePort.Name()).Neighbor(nbrInterfaceID) // *telemetry.Lldp_Interface_NeighborPath
	gotNbrState := nbrStatePath.Get(t)                                                       // *telemetry.Lldp_Interface_Neighbor which is a ValidatedGoStruct.
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
