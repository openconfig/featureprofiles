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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

type lldpTestParameters struct {
	systemName string
	macAddress string
	otgName    string
}

type lldpNeighbors struct {
	systemName    string
	portId        string
	portIdType    otgtelemetry.E_LldpNeighbor_PortIdType
	chassisId     string
	chassisIdType otgtelemetry.E_LldpNeighbor_ChassisIdType
}

const (
	portName     = "port1"
	lldpEnabled  = true
	lldpDisabled = false
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
		systemName: "ixia-otg",
		macAddress: "02:00:22:01:01:01",
		otgName:    "ixia-otg",
	}
)

// TestLLDPEnabled tests LLDP advertisement turned on.
func TestLLDPEnabled(t *testing.T) {
	// DUT configuration.
	t.Log("Configure DUT.")
	dut, dutConf := configureDUT(t, "dut", lldpEnabled)
	disableP4RTLLDP(t, dut)
	dutPort := dut.Port(t, portName)
	verifyNodeConfig(t, dut, dutPort, dutConf, lldpEnabled)

	// ATE Configuration.
	t.Log("Configure ATE.")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	otgConfig := configureATE(t, otg)

	checkLLDPMetricsOTG(t, otg, otgConfig, lldpEnabled)

	dutPeerState := lldpNeighbors{
		systemName:    lldpSrc.systemName,
		chassisId:     lldpSrc.macAddress,
		chassisIdType: otgtelemetry.LldpNeighbor_ChassisIdType_MAC_ADDRESS,
		portId:        ate.Port(t, portName).Name(),
		portIdType:    otgtelemetry.LldpNeighbor_PortIdType_INTERFACE_NAME,
	}
	verifyDUTTelemetry(t, dut, dutPort, dutConf, dutPeerState)

	expOtgLLDPNeighbor := lldpNeighbors{
		systemName:    dutConf.GetSystemName(),
		portId:        dutPort.Name(),
		portIdType:    otgtelemetry.LldpNeighbor_PortIdType_INTERFACE_NAME,
		chassisId:     strings.ToUpper(dutConf.GetChassisId()),
		chassisIdType: otgtelemetry.E_LldpNeighbor_ChassisIdType(dutConf.GetChassisIdType()),
	}
	checkOTGLLDPNeighbor(t, otg, otgConfig, expOtgLLDPNeighbor)

	// disable LLDP before releasing the devices.
	gnmi.Replace(t, dut, gnmi.OC().Lldp().Enabled().Config(), false)
}

// TestLLDPDisabled tests LLDP advertisement turned off.
func TestLLDPDisabled(t *testing.T) {
	// DUT configuration.
	t.Log("Configure DUT.")
	dut, dutConf := configureDUT(t, "dut", lldpDisabled)
	disableP4RTLLDP(t, dut)
	dutPort := dut.Port(t, portName)
	verifyNodeConfig(t, dut, dutPort, dutConf, lldpDisabled)

	// ATE Configuration.
	t.Log("Configure ATE.")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	otgConfig := configureATE(t, otg)

	checkLLDPMetricsOTG(t, otg, otgConfig, lldpDisabled)
	expOtgLLDPNeighbor := lldpNeighbors{}
	checkOTGLLDPNeighbor(t, otg, otgConfig, expOtgLLDPNeighbor)
}

// configureDUT configures LLDP on a single node.
func configureDUT(t *testing.T, name string, lldpEnabled bool) (*ondatra.DUTDevice, *oc.Lldp) {
	node := ondatra.DUT(t, name)
	p := node.Port(t, portName)
	lldp := gnmi.OC().Lldp()

	gnmi.Replace(t, node, lldp.Enabled().Config(), lldpEnabled)

	if lldpEnabled {
		gnmi.Replace(t, node, lldp.Interface(p.Name()).Enabled().Config(), lldpEnabled)
	}
	if deviations.InterfaceEnabled(node) {
		gnmi.Replace(t, node, gnmi.OC().Interface(p.Name()).Enabled().Config(), true)
	}

	return node, gnmi.GetConfig(t, node, lldp.Config())
}

func configureATE(t *testing.T, otg *otg.OTG) gosnappi.Config {

	// Device configuration + Ethernet configuration.
	config := gosnappi.NewConfig()
	srcPort := config.Ports().Add().SetName(portName)
	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(srcPort.Name())

	// LLDP configuration.
	lldp := config.Lldp().Add()
	lldp.SystemName().SetValue(lldpSrc.systemName)
	lldp.SetName(lldpSrc.otgName)
	lldp.Connection().SetPortName(portName)
	lldp.ChassisId().MacAddressSubtype().SetValue(lldpSrc.macAddress)

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

// checkLLDPMetricsOTG verifies OTG side lldp Metrics values based on DUT side lldp is enabled or not
func checkLLDPMetricsOTG(t *testing.T, otg *otg.OTG, c gosnappi.Config, lldpEnabled bool) {
	for _, lldp := range c.Lldp().Items() {
		lastValue, ok := gnmi.Watch(t, otg, gnmi.OTG().LldpInterface(lldp.Name()).Counters().FrameOut().State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
			txPackets, _ := v.Val()
			return v.IsPresent() && txPackets != 0
		}).Await(t)
		if !ok {
			txPackets, _ := lastValue.Val()
			t.Errorf("LLDP sent packets got: %v, want: > 0.", txPackets)
		}
		framesIn, _ := gnmi.Watch(t, otg, gnmi.OTG().LldpInterface(lldp.Name()).Counters().FrameIn().State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
			time.Sleep(1 * time.Second)
			return v.IsPresent()
		}).Await(t)
		otgutils.LogLLDPMetrics(t, otg, c)
		if lldpEnabled {
			if rxPackets, _ := framesIn.Val(); rxPackets == 0 {
				t.Errorf("LLDP received packets got: %v, want: > 0.", rxPackets)
			}
		} else {
			if rxPackets, _ := framesIn.Val(); rxPackets != 0 {
				t.Errorf("LLDP received packets got: %v, want: 0.", rxPackets)
			}
		}
	}
}

// checkOTGLLDPNeighbor verifies OTG side lldp neighbor states
func checkOTGLLDPNeighbor(t *testing.T, otg *otg.OTG, c gosnappi.Config, expLldpNeighbor lldpNeighbors) {
	otgutils.LogLLDPNeighborStates(t, otg, c)

	lldpState := gnmi.Lookup(t, otg, gnmi.OTG().LldpInterface(lldpSrc.otgName).LldpNeighborDatabase().State())
	v, isPresent := lldpState.Val()
	if isPresent {
		neighbors := v.LldpNeighbor
		neighborFound := false
		for _, neighbor := range neighbors {
			if expLldpNeighbor.Equal(neighbor) {
				neighborFound = true
				break
			}
		}
		if !neighborFound {
			t.Errorf("LLDP Neighbor not found")
		}
	}
}

// verifyDUTTelemetry verifies the telemetry values from the node such as port LLDP neighbor info.
func verifyDUTTelemetry(t *testing.T, dut *ondatra.DUTDevice, nodePort *ondatra.Port, conf *oc.Lldp, dutPeerState lldpNeighbors) {
	verifyNodeConfig(t, dut, nodePort, conf, true)
	interfacePath := gnmi.OC().Lldp().Interface(nodePort.Name())

	// Ensure that DUT does not generate any LLDP messages irrespective of the
	// configuration of lldp/interfaces/interface/config/enabled (TRUE or FALSE)
	// on any interface.
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

func (expLldpNeighbor *lldpNeighbors) Equal(neighbour *otgtelemetry.LldpInterface_LldpNeighborDatabase_LldpNeighbor) bool {
	return neighbour.GetChassisId() == expLldpNeighbor.chassisId &&
		neighbour.GetChassisIdType() == expLldpNeighbor.chassisIdType &&
		neighbour.GetPortId() == expLldpNeighbor.portId &&
		neighbour.GetPortIdType() == expLldpNeighbor.portIdType &&
		neighbour.GetSystemName() == expLldpNeighbor.systemName
}

func disableP4RTLLDP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := `p4-runtime
					shutdown`
		if _, err := dut.RawAPIs().GNMI(t).
			Set(context.Background(), cliSetRequest(cli)); err != nil {
			t.Fatalf("Failed to disable P4RTLLDP: %v", err)
		}
	}
}

func cliSetRequest(config string) *gpb.SetRequest {
	return &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
}
