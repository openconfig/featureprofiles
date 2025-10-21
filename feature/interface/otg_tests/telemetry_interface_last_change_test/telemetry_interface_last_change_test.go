// Copyright 2025 Google LLC
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

package telemetry_interface_last_change_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4Address    = "192.168.1.1"
	ipv6Address    = "2001:DB8::1"
	ipv4LagAddress = "192.168.20.1"
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	awaitTimeout   = 2 * time.Minute
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type lastChangeTestCase struct {
	desc      string
	intfName  string // The interface or subinterface name.
	initialLC uint64
	finalLC   uint64
}

func configureDUTLAG(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.SetType(oc.IETFInterfaces_InterfaceType_ieee8023adLag)
	agg.GetOrCreateAggregation().SetLagType(oc.IfAggregate_AggregationType_STATIC)
	subIntf := agg.GetOrCreateSubinterface(0)
	subIntf.SetEnabled(true)
	if !deviations.IPv4MissingEnabled(dut) {
		subIntf.GetOrCreateIpv4().Enabled = ygot.Bool(true)
	}
	a := subIntf.GetOrCreateIpv4().GetOrCreateAddress(ipv4LagAddress)
	a.SetPrefixLength(ipv4PrefixLen)

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		if deviations.FrBreakoutFix(dut) && port.PMD() == ondatra.PMD100GBASEFR {
			i.GetOrCreateEthernet().SetAutoNegotiate(false)
			i.GetOrCreateEthernet().SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			i.GetOrCreateEthernet().SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
		}
		i.GetOrCreateEthernet().SetAggregateId(aggID)
		i.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

		if deviations.InterfaceEnabled(dut) {
			i.SetEnabled(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func TestLAGInterfaceLastChangeState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutAggPorts := []*ondatra.Port{
		dut.Port(t, "port1"),
	}
	aggID := netutil.NextAggregateInterface(t, dut)
	configureDUTLAG(t, dut, dutAggPorts, aggID)

	aggIntfPath := gnmi.OC().Interface(aggID)

	gnmi.Update(t, dut, aggIntfPath.Enabled().Config(), true)
	gnmi.Await(t, dut, aggIntfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)

	initialIntfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.LastChange().State()).Val()
	if !present {
		t.Errorf("Failed to lookup initial LastChange for interface %s", aggID)
	}
	initialSubintfLCVal, present := gnmi.Lookup(t, dut, aggIntfPath.Subinterface(0).LastChange().State()).Val()
	if !present {
		t.Errorf("Failed to lookup initial LastChange for subinterface %s:0", aggID)
	}

	gnmi.Update(t, dut, aggIntfPath.Enabled().Config(), false)
	if dut.Vendor() == ondatra.JUNIPER {
		gnmi.Await(t, dut, aggIntfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_LOWER_LAYER_DOWN)
	} else {
		gnmi.Await(t, dut, aggIntfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_DOWN)
	}

	finalIntfLCVal, finalIntfPresent := gnmi.Lookup(t, dut, aggIntfPath.LastChange().State()).Val()
	if !finalIntfPresent {
		t.Errorf("Failed to lookup final LastChange for interface %s", aggID)
	}
	finalSubintfLCVal, finalSubintfPresent := gnmi.Lookup(t, dut, aggIntfPath.Subinterface(0).LastChange().State()).Val()
	if !finalSubintfPresent {
		t.Errorf("Failed to lookup final LastChange for subinterface %s:0", aggID)
	}

	cases := []lastChangeTestCase{{
		desc:      "Interface_LastChange",
		intfName:  aggID,
		initialLC: initialIntfLCVal,
		finalLC:   finalIntfLCVal,
	}, {
		desc:      "Subinterface_LastChange",
		intfName:  fmt.Sprintf("%s:%d", aggID, 0),
		initialLC: initialSubintfLCVal,
		finalLC:   finalSubintfLCVal,
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.finalLC <= tc.initialLC {
				t.Errorf("%s: LastChange timestamp did not increase after link flap, initial: %d, final: %d for interface/sub-interface: %s", tc.desc, tc.initialLC, tc.finalLC, tc.intfName)
			} else {
				t.Logf("%s: LastChange timestamp increased as expected, initial: %d, final: %d for interface/sub-interface: %s", tc.desc, tc.initialLC, tc.finalLC, tc.intfName)
			}
		})
	}
}

func configureInterface(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) {
	t.Helper()
	i := &oc.Interface{
		Name:        ygot.String(port.Name()),
		Description: ygot.String(fmt.Sprintf("Description for %s", port.Name())),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	i.GetOrCreateEthernet()
	i.SetEnabled(true)

	s := i.GetOrCreateSubinterface(0)
	s.SetEnabled(true)

	v4 := s.GetOrCreateIpv4()
	if !deviations.IPv4MissingEnabled(dut) {
		v4.Enabled = ygot.Bool(true)
	}
	a4 := v4.GetOrCreateAddress(ipv4Address)
	a4.SetPrefixLength(ipv4PrefixLen)

	v6 := s.GetOrCreateIpv6()
	v6.SetEnabled(true)
	a6 := v6.GetOrCreateAddress(ipv6Address)
	a6.SetPrefixLength(ipv6PrefixLen)

	gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
}

func TestEthernetInterfaceLastChangeState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	port := dut.Port(t, "port1")

	configureInterface(t, dut, port)
	intfPath := gnmi.OC().Interface(port.Name())
	gnmi.Await(t, dut, intfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_UP)

	initialIntfLCVal, present := gnmi.Lookup(t, dut, intfPath.LastChange().State()).Val()
	if !present {
		t.Errorf("Failed to lookup initial LastChange for interface %s", port.Name())
	}
	initialSubintfLCVal, present := gnmi.Lookup(t, dut, intfPath.Subinterface(0).LastChange().State()).Val()
	if !present {
		t.Errorf("Failed to lookup initial LastChange for subinterface %s:0", port.Name())
	}

	gnmi.Update(t, dut, intfPath.Enabled().Config(), false)
	gnmi.Await(t, dut, intfPath.OperStatus().State(), awaitTimeout, oc.Interface_OperStatus_DOWN)

	finalIntfLCVal, finalIntfPresent := gnmi.Lookup(t, dut, intfPath.LastChange().State()).Val()
	if !finalIntfPresent {
		t.Errorf("Failed to lookup final LastChange for interface %s", port.Name())
	}
	finalSubintfLCVal, finalSubintfPresent := gnmi.Lookup(t, dut, intfPath.Subinterface(0).LastChange().State()).Val()
	if !finalSubintfPresent {
		t.Errorf("Failed to lookup final LastChange for subinterface %s:0", port.Name())
	}

	cases := []lastChangeTestCase{{
		desc:      "Interface_LastChange",
		intfName:  port.Name(),
		initialLC: initialIntfLCVal,
		finalLC:   finalIntfLCVal,
	}, {
		desc:      "Subinterface_LastChange",
		intfName:  fmt.Sprintf("%s:%d", port.Name(), 0),
		initialLC: initialSubintfLCVal,
		finalLC:   finalSubintfLCVal,
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.finalLC <= tc.initialLC {
				t.Errorf("%s: LastChange timestamp did not increase after link flap, initial: %d, final: %d for interface/sub-interface: %s", tc.desc, tc.initialLC, tc.finalLC, tc.intfName)
			} else {
				t.Logf("%s: LastChange timestamp increased as expected, initial: %d, final: %d for interface/sub-interface: %s", tc.desc, tc.initialLC, tc.finalLC, tc.intfName)
			}
		})
	}
}
